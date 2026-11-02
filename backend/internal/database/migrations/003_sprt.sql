-- Optional sequential probability ratio test. When it accepts a hypothesis
-- the tournament stops early and stores the outcome.
ALTER TABLE tournaments
    ADD COLUMN sprt_elo0 DOUBLE PRECISION,
    ADD COLUMN sprt_elo1 DOUBLE PRECISION,
    ADD COLUMN sprt_alpha DOUBLE PRECISION,
    ADD COLUMN sprt_beta DOUBLE PRECISION,
    ADD COLUMN sprt_result TEXT CHECK (sprt_result IN ('H0', 'H1')),
    ADD CONSTRAINT tournaments_sprt_check CHECK (num_nulls(sprt_elo0, sprt_elo1, sprt_alpha, sprt_beta) IN (0, 4));
