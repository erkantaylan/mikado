-- Hosts the server answers besides localhost, added at runtime with
-- `mikado hosts add` (those from --allow-host / $MIKADO_ALLOWED_HOSTS are not
-- stored). A name is a host ("mikado.home") or a wildcard ("*.ts.net").
CREATE TABLE hosts (
    name     TEXT PRIMARY KEY,
    added_at TEXT NOT NULL
);
