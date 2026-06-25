-- +goose Up
-- Per-bond coupon disposition (#66): does the coupon pay out to the bank account
-- (the common Indonesian govt-primary case) or accrue inside the instrument
-- (secondary-market / some structured holdings)? Until now the accrued-interest
-- snapshot dialog carried a single global accrued=0 default; this promotes the
-- choice to a per-bond enum so the form can pivot on it. Additive: every existing
-- row is born 'pays_out', which reproduces today's default behaviour exactly.
ALTER TABLE public.bond_details
    ADD COLUMN coupon_disposition text NOT NULL DEFAULT 'pays_out'
    CONSTRAINT bond_details_coupon_disposition_check
        CHECK (coupon_disposition = ANY (ARRAY['pays_out'::text, 'accrues'::text]));

-- +goose Down
ALTER TABLE public.bond_details DROP COLUMN coupon_disposition;
