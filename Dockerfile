# Minimal runtime image from the prebuilt static binary:
#   make web linux && docker build --platform linux/amd64 -t loadshop .
FROM gcr.io/distroless/static-debian12:nonroot
COPY bin/loadshop-linux-amd64 /loadshop
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/loadshop"]
CMD ["-addr", ":8000"]
