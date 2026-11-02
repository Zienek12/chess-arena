-- A name for each starting position, such as its opening, taken from the
-- PGN tags or the FEN line it was imported from.
ALTER TABLE positions ADD COLUMN name TEXT;
