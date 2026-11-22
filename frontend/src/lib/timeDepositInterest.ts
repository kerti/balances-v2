import { subtractDecimals } from "@/lib/decimal";

// "Of which interest" for a time deposit snapshot (#844): the Value the member
// types from the statement minus the principal in force. Display only — the
// server derives and stores the real accrued_interest from the same two figures,
// so this never reaches a payload. Exact decimal subtraction (lib/decimal), so
// float noise ("1234.56 − 1000.5") never prints or flips the below-principal test.
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
  const interest = principal == null ? null : subtractDecimals(value, principal);
  if (interest === null) return { valid: false, amount: "", below: false };
  return { valid: true, amount: interest, below: interest.startsWith("-") };
}
