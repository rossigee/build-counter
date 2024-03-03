CREATE TABLE builds (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    build_id VARCHAR(255) NOT NULL,
    started TIMESTAMP NOT NULL,
    finished TIMESTAMP
);
