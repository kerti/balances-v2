// "Of which interest" for a time deposit snapshot (#844): the Value the member
// types from the statement minus the principal in force. Display only — the
// server derives and stores the real accrued_interest from the same two figures,
// so this never reaches a payload. Computed at the 4dp money scale so float noise
// ("1234.56 − 1000.5") never prints or flips the below-principal test.
export type TimeDepositInterest = {
  valid: boolean;
  // The derived interest as a plain decimal string, "" while not valid.
  amount: string;
  // Value is below the principal: a soft warning, never a block on saving.
  below: boolean;
};

export function timeDepositInterest(
  value: string,
  principal: string | null | undefined,
): TimeDepositInterest {
  const v = Number(value);
  const p = Number(principal);
  if (
    !value.trim() ||
    principal == null ||
    !principal.trim() ||
    Number.isNaN(v) ||
    Number.isNaN(p)
  ) {
    return { valid: false, amount: "", below: false };
  }
  const interest = Number((v - p).toFixed(4));
  return { valid: true, amount: interest.toString(), below: interest < 0 };
}
