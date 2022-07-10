CREATE TABLE IF NOT EXISTS "decks" (
        "id" INTEGER
                PRIMARY KEY
                NOT NULL,
        "name" TEXT
                NOT NULL
);

CREATE TABLE IF NOT EXISTS "cards" (
        "id" INTEGER
                PRIMARY KEY
                NOT NULL,
        "deck" INTEGER
                NOT NULL
                REFERENCES "decks" ("id")
                ON UPDATE CASCADE
                ON DELETE CASCADE,
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