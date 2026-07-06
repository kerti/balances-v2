import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { postCreateImport, type CreateImportArgs } from "@/hooks/snapshotImport";
import type {
  Bond,
  RiskProfile,
  BondListItem,
  BondType,
  CouponFrequency,
  CouponDisposition,
  Gold,
  GoldListItem,
  MutualFund,
  MutualFundListItem,
  MutualFundType,
  RolloverPolicy,
  Stock,
  StockListItem,
  TimeDeposit,
  TimeDepositListItem,
} from "@/api/types";

// ----- stock -----------------------------------------------------------

export type CreateStockPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  native_currency: string;
  ticker: string;
  exchange: string;
};

export type UpdateStockPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  ticker: string;
  exchange: string;
};

export function useStocks() {
  return useQuery({
    queryKey: ["stocks"],
    queryFn: () => api<StockListItem[]>("/api/investments/stocks"),
    staleTime: 10_000,
  });
}

export function useStock(id: string | null) {
  return useQuery({
    queryKey: ["stocks", id],
    queryFn: () => api<Stock>(`/api/investments/stocks/${id}`),
    enabled: !!id,
  });
}

export function useCreateStock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateStockPayload) =>
      api<Stock>("/api/investments/stocks", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stocks"] });
    },
  });
}

export function useUpdateStock(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateStockPayload) =>
      api<Stock>(`/api/investments/stocks/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stocks"] });
      qc.invalidateQueries({ queryKey: ["stocks", id] });
    },
  });
}

export function useDeleteStock() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/investments/stocks/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stocks"] });
    },
  });
}

// ----- mutual fund -----------------------------------------------------

export type CreateMutualFundPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  native_currency: string;
  fund_code: string;
  fund_manager: string | null;
  fund_type: MutualFundType;
};

export type UpdateMutualFundPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  fund_code: string;
  fund_manager: string | null;
  fund_type: MutualFundType;
};

export function useMutualFunds() {
  return useQuery({
    queryKey: ["mutual-funds"],
    queryFn: () => api<MutualFundListItem[]>("/api/investments/mutual-funds"),
    staleTime: 10_000,
  });
}

export function useMutualFund(id: string | null) {
  return useQuery({
    queryKey: ["mutual-funds", id],
    queryFn: () => api<MutualFund>(`/api/investments/mutual-funds/${id}`),
    enabled: !!id,
  });
}

export function useCreateMutualFund() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateMutualFundPayload) =>
      api<MutualFund>("/api/investments/mutual-funds", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["mutual-funds"] });
    },
  });
}

export function useUpdateMutualFund(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateMutualFundPayload) =>
      api<MutualFund>(`/api/investments/mutual-funds/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["mutual-funds"] });
      qc.invalidateQueries({ queryKey: ["mutual-funds", id] });
    },
  });
}

export function useDeleteMutualFund() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/investments/mutual-funds/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["mutual-funds"] });
    },
  });
}

// ----- gold ------------------------------------------------------------

export type GoldForm = "bar" | "coin" | "digital" | "jewelry";

export type CreateGoldPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  native_currency: string;
  form: GoldForm;
  purity: string;
};

export type UpdateGoldPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  form: GoldForm;
  purity: string;
};

export function useGolds() {
  return useQuery({
    queryKey: ["golds"],
    queryFn: () => api<GoldListItem[]>("/api/investments/golds"),
    staleTime: 10_000,
  });
}

export function useGold(id: string | null) {
  return useQuery({
    queryKey: ["golds", id],
    queryFn: () => api<Gold>(`/api/investments/golds/${id}`),
    enabled: !!id,
  });
}

export function useCreateGold() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateGoldPayload) =>
      api<Gold>("/api/investments/golds", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["golds"] });
    },
  });
}

export function useUpdateGold(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateGoldPayload) =>
      api<Gold>(`/api/investments/golds/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["golds"] });
      qc.invalidateQueries({ queryKey: ["golds", id] });
    },
  });
}

export function useDeleteGold() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/investments/golds/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["golds"] });
    },
  });
}

// ----- bond ------------------------------------------------------------

export type CreateBondPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  native_currency: string;
  bond_type: BondType;
  series_code: string | null;
  issuer: string;
  // face_value + placement_date seed the placement Buy for a govt_primary bond
  // (issue #27): nominal placed at par. Omitted for secondary_market, where the
  // user records the actual Buy. Required only for govt_primary.
  face_value: string;
  placement_date: string;
  coupon_rate: string;
  coupon_frequency: CouponFrequency;
  coupon_disposition: CouponDisposition;
  maturity_date: string;
};

export type UpdateBondPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  bond_type: BondType;
  series_code: string | null;
  issuer: string;
  coupon_rate: string;
  coupon_frequency: CouponFrequency;
  coupon_disposition: CouponDisposition;
  maturity_date: string;
};

export function useBonds() {
  return useQuery({
    queryKey: ["bonds"],
    queryFn: () => api<BondListItem[]>("/api/investments/bonds"),
    staleTime: 10_000,
  });
}

export function useBond(id: string | null) {
  return useQuery({
    queryKey: ["bonds", id],
    queryFn: () => api<Bond>(`/api/investments/bonds/${id}`),
    enabled: !!id,
  });
}

export function useCreateBond() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateBondPayload) =>
      api<Bond>("/api/investments/bonds", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["bonds"] });
    },
  });
}

export function useUpdateBond(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateBondPayload) =>
      api<Bond>(`/api/investments/bonds/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["bonds"] });
      qc.invalidateQueries({ queryKey: ["bonds", id] });
    },
  });
}

export function useDeleteBond() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/investments/bonds/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["bonds"] });
    },
  });
}

// ----- time deposit ----------------------------------------------------

export type CreateTimeDepositPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  native_currency: string;
  bank_name: string;
  principal: string;
  interest_rate: string;
  term_months: number;
  placement_date: string;
  maturity_date: string;
  rollover_policy: RolloverPolicy;
  // Links a rollover successor back to its matured source (issue #29). Omitted
  // for a fresh deposit.
  rolled_from_investment_id?: string;
};

export type UpdateTimeDepositPayload = {
  display_name: string;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile;
  bank_name: string;
  principal: string;
  interest_rate: string;
  term_months: number;
  placement_date: string;
  maturity_date: string;
  rollover_policy: RolloverPolicy;
};

export function useTimeDeposits() {
  return useQuery({
    queryKey: ["time-deposits"],
    queryFn: () => api<TimeDepositListItem[]>("/api/investments/time-deposits"),
    staleTime: 10_000,
  });
}

export function useTimeDeposit(id: string | null) {
  return useQuery({
    queryKey: ["time-deposits", id],
    queryFn: () => api<TimeDeposit>(`/api/investments/time-deposits/${id}`),
    enabled: !!id,
  });
}

export function useCreateTimeDeposit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateTimeDepositPayload) =>
      api<TimeDeposit>("/api/investments/time-deposits", {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: (_data, variables) => {
      qc.invalidateQueries({ queryKey: ["time-deposits"] });
      // Refresh the source TD so its rolled_to ref resolves (and the rollover
      // callout disappears) without a manual reload (issue #29).
      if (variables.rolled_from_investment_id) {
        qc.invalidateQueries({
          queryKey: ["time-deposits", variables.rolled_from_investment_id],
        });
      }
    },
  });
}

export function useUpdateTimeDeposit(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: UpdateTimeDepositPayload) =>
      api<TimeDeposit>(`/api/investments/time-deposits/${id}`, {
        method: "PATCH",
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["time-deposits"] });
      qc.invalidateQueries({ queryKey: ["time-deposits", id] });
    },
  });
}

// Manually link a hand-created successor so the matured source's rollover
// callout clears (issue #65). sourceId is the matured deposit being viewed;
// the payload's successor_id is the existing deposit that holds the rolled
// funds. Invalidates both detail queries + the list so the callout and the
// rollover chain refresh without a reload.
export function useLinkRolloverSuccessor(sourceId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (successorId: string) =>
      api<TimeDeposit>(`/api/investments/time-deposits/${sourceId}/rollover-successor`, {
        method: "POST",
        body: JSON.stringify({ successor_id: successorId }),
      }),
    onSuccess: (_data, successorId) => {
      qc.invalidateQueries({ queryKey: ["time-deposits"] });
      qc.invalidateQueries({ queryKey: ["time-deposits", sourceId] });
      qc.invalidateQueries({ queryKey: ["time-deposits", successorId] });
    },
  });
}

export function useDeleteTimeDeposit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/api/investments/time-deposits/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["time-deposits"] });
    },
  });
}

// Create-from-list import for the five investment subtypes (issue #90): a new
// position from an uploaded workbook (Detail + Snapshots + Transactions ledger),
// a preview being a server-side dry-run. Only a committed create refreshes the
// subtype's list. Each subtype posts to its own /import endpoint; the dialog +
// transport are group-agnostic (shared with the asset/liability/receivable
// groups via postCreateImport).
function useImportCreateInvestment(plural: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport(`/api/investments/${plural}`, args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: [plural] });
      }
    },
  });
}

export function useImportCreateStock() {
  return useImportCreateInvestment("stocks");
}

export function useImportCreateMutualFund() {
  return useImportCreateInvestment("mutual-funds");
}

export function useImportCreateGold() {
  return useImportCreateInvestment("golds");
}

export function useImportCreateBond() {
  return useImportCreateInvestment("bonds");
}

export function useImportCreateTimeDeposit() {
  return useImportCreateInvestment("time-deposits");
}
