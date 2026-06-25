-- name: CreateBondDetails :one
INSERT INTO bond_details (
    investment_id, bond_type, series_code, issuer,
    coupon_rate, coupon_frequency, maturity_date, coupon_disposition
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetBondDetailsByInvestmentID :one
SELECT *
FROM bond_details
WHERE investment_id = $1;

-- name: ListBondDetailsByInvestmentIDs :many
SELECT *
FROM bond_details
WHERE investment_id = ANY($1::uuid[]);

-- name: UpdateBondDetails :one
UPDATE bond_details
SET bond_type          = $2,
    series_code        = $3,
    issuer             = $4,
    coupon_rate        = $5,
    coupon_frequency   = $6,
    maturity_date      = $7,
    coupon_disposition = $8
WHERE investment_id = $1
RETURNING *;
