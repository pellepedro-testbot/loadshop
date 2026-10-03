package shop

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	numShards       = 64
	maxLineQuantity = 999 // accumulated quantity per cart line is clamped here
	taxRateBP       = 800 // 8.00% in basis points
	freeShipCents   = 5000
	shippingCents   = 999
)

var (
	errEmptyCart = errors.New("cart is empty")
	errNotInCart = errors.New("product is not in the cart")
	errNoOrder   = errors.New("order not found")
)

type cartLine struct {
	productID int
	quantity  int
}

type userState struct {
	cart   []cartLine // insertion order
	orders []*Order   // oldest first
}

type shard struct {
	mu    sync.Mutex
	users map[string]*userState
}

// Store holds per-user carts and orders in memory, sharded by username hash.
// There is deliberately no expiry: state lives until reset or restart.
type Store struct {
	catalog  *Catalog
	shards   [numShards]shard
	orderSeq atomic.Int64
	now      func() time.Time
}

func NewStore(c *Catalog) *Store {
	s := &Store{catalog: c, now: time.Now}
	for i := range s.shards {
		s.shards[i].users = make(map[string]*userState)
	}
	return s
}

func (s *Store) shard(user string) *shard {
	// FNV-1a, inlined to avoid allocations.
	h := uint32(2166136261)
	for i := 0; i < len(user); i++ {
		h ^= uint32(user[i])
		h *= 16777619
	}
	return &s.shards[h%numShards]
}

// withUser runs fn with the user's state locked (created on demand).
func (s *Store) withUser(user string, fn func(u *userState)) {
	sh := s.shard(user)
	sh.mu.Lock()
	u := sh.users[user]
	if u == nil {
		u = &userState{}
		sh.users[user] = u
	}
	fn(u)
	sh.mu.Unlock()
}

// Reset drops all carts and orders.
func (s *Store) Reset() {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		sh.users = make(map[string]*userState)
		sh.mu.Unlock()
	}
}

// CartLine is the API view of a cart line.
type CartLine struct {
	ProductID int     `json:"productId"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Brand     string  `json:"brand"`
	Image     string  `json:"image"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	LineTotal float64 `json:"lineTotal"`
}

// Totals are monetary summaries in dollars.
type Totals struct {
	ItemCount    int     `json:"itemCount"`
	Subtotal     float64 `json:"subtotal"`
	Tax          float64 `json:"tax"`
	ShippingCost float64 `json:"shippingCost"`
	Total        float64 `json:"total"`
}

// Cart is the API view of a cart.
type Cart struct {
	Items []CartLine `json:"items"`
	Totals
}

// Shipping holds checkout address details.
type Shipping struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Address string `json:"address"`
	City    string `json:"city"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}

// Order is an immutable record of a checkout.
type Order struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	Items        []CartLine `json:"items"`
	Shipping     Shipping   `json:"shipping"`
	PaymentLast4 string     `json:"paymentLast4"`
	Totals
}

func dollars(c int64) float64 { return float64(c) / 100 }

func (s *Store) view(lines []cartLine) Cart {
	cart := Cart{Items: make([]CartLine, 0, len(lines))}
	var sub int64
	for _, l := range lines {
		p := s.catalog.Get(l.productID)
		lt := p.priceCents * int64(l.quantity)
		sub += lt
		cart.ItemCount += l.quantity
		cart.Items = append(cart.Items, CartLine{
			ProductID: p.ID, Slug: p.Slug, Name: p.Name, Brand: p.Brand, Image: p.Image,
			Price: p.Price, Quantity: l.quantity, LineTotal: dollars(lt),
		})
	}
	tax := (sub*taxRateBP + 5000) / 10000
	var ship int64
	if sub > 0 && sub < freeShipCents {
		ship = shippingCents
	}
	cart.Subtotal, cart.Tax, cart.ShippingCost, cart.Total = dollars(sub), dollars(tax), dollars(ship), dollars(sub+tax+ship)
	return cart
}

// GetCart returns the user's cart.
func (s *Store) GetCart(user string) (c Cart) {
	s.withUser(user, func(u *userState) { c = s.view(u.cart) })
	return
}

// AddItem adds qty of a product (accumulating onto an existing line).
func (s *Store) AddItem(user string, productID, qty int) (c Cart) {
	s.withUser(user, func(u *userState) {
		for i := range u.cart {
			if u.cart[i].productID == productID {
				u.cart[i].quantity = min(u.cart[i].quantity+qty, maxLineQuantity)
				c = s.view(u.cart)
				return
			}
		}
		u.cart = append(u.cart, cartLine{productID, qty})
		c = s.view(u.cart)
	})
	return
}

// SetQuantity sets a line's quantity; 0 removes it.
func (s *Store) SetQuantity(user string, productID, qty int) (c Cart, err error) {
	s.withUser(user, func(u *userState) {
		for i := range u.cart {
			if u.cart[i].productID == productID {
				if qty == 0 {
					u.cart = append(u.cart[:i], u.cart[i+1:]...)
				} else {
					u.cart[i].quantity = qty
				}
				c = s.view(u.cart)
				return
			}
		}
		err = errNotInCart
	})
	return
}

// RemoveItem deletes a line. Removing an absent line is not an error (idempotent).
func (s *Store) RemoveItem(user string, productID int) (c Cart) {
	s.withUser(user, func(u *userState) {
		for i := range u.cart {
			if u.cart[i].productID == productID {
				u.cart = append(u.cart[:i], u.cart[i+1:]...)
				break
			}
		}
		c = s.view(u.cart)
	})
	return
}

// Checkout converts the cart into a confirmed order and empties the cart.
func (s *Store) Checkout(user string, ship Shipping, last4 string) (o *Order, err error) {
	s.withUser(user, func(u *userState) {
		if len(u.cart) == 0 {
			err = errEmptyCart
			return
		}
		v := s.view(u.cart)
		o = &Order{
			ID:           fmt.Sprintf("ORD-%06d", s.orderSeq.Add(1)),
			Username:     user,
			Status:       "confirmed",
			CreatedAt:    s.now().UTC().Truncate(time.Second),
			Items:        v.Items,
			Shipping:     ship,
			PaymentLast4: last4,
			Totals:       v.Totals,
		}
		u.orders = append(u.orders, o)
		u.cart = nil
	})
	return
}

// Orders returns the user's orders, newest first.
func (s *Store) Orders(user string) (out []*Order) {
	s.withUser(user, func(u *userState) {
		out = make([]*Order, len(u.orders))
		for i, o := range u.orders {
			out[len(u.orders)-1-i] = o
		}
	})
	return
}

// Order returns one of the user's orders.
func (s *Store) Order(user, id string) (o *Order, err error) {
	s.withUser(user, func(u *userState) {
		for _, x := range u.orders {
			if x.ID == id {
				o = x
				return
			}
		}
		err = errNoOrder
	})
	return
}
