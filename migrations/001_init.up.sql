CREATE TABLE IF NOT EXISTS "cards" (
        "id" INTEGER
                PRIMARY KEY
                NOT NULL,
        "front" TEXT
                NOT NULL,
        "back" TEXT
                NOT NULL,
        "last_practiced" INTEGER
                DEFAULT NULL,
        "comfort" FLOAT
                NOT NULL
                DEFAULT -1
                CHECK ("comfort" = -1 OR ("comfort" BETWEEN 0 AND 4))
);