FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-slack"]
COPY baton-slack /