package shop

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

//go:embed seed.json
var seedJSON []byte

// Spec is a key/value product attribute.
type Spec struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Product is an immutable catalogue entry.
type Product struct {
	ID               int     `json:"id"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Brand            string  `json:"brand"`
	Category         string  `json:"category"`
	CategorySlug     string  `json:"categorySlug"`
	Price            float64 `json:"price"`
	Rating           float64 `json:"rating"`
	ReviewCount      int     `json:"reviewCount"`
	ShortDescription string  `json:"shortDescription"`
	Description      string  `json:"description"`
	Specs            []Spec  `json:"specs"`
	Stock            int     `json:"stock"`
	Image            string  `json:"image"`

	priceCents int64
	search     string // lowercased name/brand/category/short description
}

// Category summarises a product category.
type Category struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Catalog is the shared, read-only catalogue built once at startup.
type Catalog struct {
	Products   []*Product // ordered by id (index = id-1)
	Categories []Category
	bySlug     map[string]*Product

	detailJSON     []*Body // index = id-1
	svg            []*Body // index = id-1
	categoriesJSON *Body

	listCache     sync.Map // key string -> *Body
	listCacheSize atomic.Int64
}

const maxListCacheEntries = 4096

// NewCatalog parses the embedded seed and precomputes the hot responses.
func NewCatalog() (*Catalog, error) {
	var products []*Product
	if err := json.Unmarshal(seedJSON, &products); err != nil {
		return nil, fmt.Errorf("parse seed: %w", err)
	}
	c := &Catalog{Products: products, bySlug: make(map[string]*Product, len(products))}
	catIndex := map[string]int{}
	for i, p := range products {
		if p.ID != i+1 {
			return nil, fmt.Errorf("seed product %q has id %d, want %d", p.Name, p.ID, i+1)
		}
		if c.bySlug[p.Slug] != nil {
			return nil, fmt.Errorf("duplicate slug %q", p.Slug)
		}
		c.bySlug[p.Slug] = p
		p.CategorySlug = slugify(p.Category)
		p.Image = fmt.Sprintf("/api/products/%d/image.svg", p.ID)
		p.priceCents = int64(math.Round(p.Price * 100))
		p.search = strings.ToLower(p.Name + " " + p.Brand + " " + p.Category + " " + p.ShortDescription)
		if idx, ok := catIndex[p.CategorySlug]; ok {
			c.Categories[idx].Count++
		} else {
			catIndex[p.CategorySlug] = len(c.Categories)
			c.Categories = append(c.Categories, Category{Slug: p.CategorySlug, Name: p.Category, Count: 1})
		}
		b, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		c.detailJSON = append(c.detailJSON, NewBody(b))
		c.svg = append(c.svg, NewBody(productSVG(p)))
	}
	cb, err := json.Marshal(c.Categories)
	if err != nil {
		return nil, err
	}
	c.categoriesJSON = NewBody(cb)
	return c, nil
}

// Get returns the product with the given id, or nil.
func (c *Catalog) Get(id int) *Product {
	if id < 1 || id > len(c.Products) {
		return nil
	}
	return c.Products[id-1]
}

// GetBySlug returns the product with the given slug, or nil.
func (c *Catalog) GetBySlug(slug string) *Product { return c.bySlug[slug] }

// ListQuery holds normalised list parameters.
type ListQuery struct {
	Limit    int
	Offset   int
	Category string
	Q        string
	Sort     string
}

func (q ListQuery) key() string {
	return fmt.Sprintf("%d|%d|%s|%s|%s", q.Limit, q.Offset, q.Category, q.Q, q.Sort)
}

// productSummary is the list-view projection (no long description/specs).
type productSummary struct {
	ID               int     `json:"id"`
	Slug             string  `json:"slug"`
	Name             string  `json:"name"`
	Brand            string  `json:"brand"`
	Category         string  `json:"category"`
	CategorySlug     string  `json:"categorySlug"`
	Price            float64 `json:"price"`
	Rating           float64 `json:"rating"`
	ReviewCount      int     `json:"reviewCount"`
	ShortDescription string  `json:"shortDescription"`
	Stock            int     `json:"stock"`
	Image            string  `json:"image"`
}

func summarize(p *Product) productSummary {
	return productSummary{p.ID, p.Slug, p.Name, p.Brand, p.Category, p.CategorySlug, p.Price,
		p.Rating, p.ReviewCount, p.ShortDescription, p.Stock, p.Image}
}

type listResponse struct {
	Items  []productSummary `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// ListJSON returns the serialised list response, cached for repeat queries.
func (c *Catalog) ListJSON(q ListQuery) *Body {
	k := q.key()
	if v, ok := c.listCache.Load(k); ok {
		return v.(*Body)
	}
	// Free-text queries are unbounded; the cap keeps memory flat under fuzzing.
	// Past it, responses are built without the up-front compression a cached
	// body pays for, so a full cache does not inflate request latency.
	if c.listCacheSize.Load() >= maxListCacheEntries {
		return newUncachedBody(c.buildList(q))
	}
	b := NewBody(c.buildList(q))
	if _, loaded := c.listCache.LoadOrStore(k, b); !loaded {
		c.listCacheSize.Add(1)
	}
	return b
}

func (c *Catalog) buildList(q ListQuery) []byte {
	matched := make([]*Product, 0, len(c.Products))
	needle := strings.ToLower(strings.TrimSpace(q.Q))
	for _, p := range c.Products {
		if q.Category != "" && p.CategorySlug != q.Category {
			continue
		}
		if needle != "" && !strings.Contains(p.search, needle) {
			continue
		}
		matched = append(matched, p)
	}
	switch q.Sort {
	case "price_asc":
		sort.SliceStable(matched, func(i, j int) bool { return matched[i].priceCents < matched[j].priceCents })
	case "price_desc":
		sort.SliceStable(matched, func(i, j int) bool { return matched[i].priceCents > matched[j].priceCents })
	case "rating":
		sort.SliceStable(matched, func(i, j int) bool { return matched[i].Rating > matched[j].Rating })
	case "name":
		sort.SliceStable(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })
	}
	total := len(matched)
	start := min(q.Offset, total)
	end := min(start+q.Limit, total)
	items := make([]productSummary, 0, end-start)
	for _, p := range matched[start:end] {
		items = append(items, summarize(p))
	}
	b, _ := json.Marshal(listResponse{Items: items, Total: total, Limit: q.Limit, Offset: q.Offset})
	return b
}

func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
