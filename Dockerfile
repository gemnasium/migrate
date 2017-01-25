FROM scratch
ADD migrater /migrate
ENTRYPOINT ["/migrate"]
