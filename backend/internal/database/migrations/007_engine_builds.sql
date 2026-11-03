-- The builds that played each game: the Docker image ID the container ran
-- and the name the engine reported with "id name". A tag such as
-- chess-arena-engine:local can be rebuilt; these columns keep every result
-- tied to the code that produced it.
ALTER TABLE games
    ADD COLUMN white_image_id TEXT,
    ADD COLUMN white_uci_name TEXT,
    ADD COLUMN black_image_id TEXT,
    ADD COLUMN black_uci_name TEXT;

ALTER TABLE game_analyses
    ADD COLUMN image_id TEXT;
