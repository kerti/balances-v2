import { useState, type ComponentType } from "react";
import { useTranslation } from "react-i18next";
import { Repeat, Sparkles } from "lucide-react";
import { EditIncomeDialog } from "@/components/dialogs/EditIncomeDialog";
import { CreateIncomeDialog, type DuplicateSeed } from "@/components/dialogs/CreateIncomeDialog";
import { ConfirmDialog } from "@/components/dialogs/ConfirmDialog";
import { useDeleteIncome } from "@/hooks/useIncome";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { formatCurrency, formatDate } from "@/lib/format";
import { ownershipLabel } from "@/lib/ownership";
import type { Income } from "@/api/types";

// The presentation-neutral projection + interaction one income row needs,
// shared by both renderers (ADR-0050 "the split lives at the renderer; the
// container is shared"). Every per-row concern that must stay renderer-
// independent — the derived labels, the edit/duplicate/delete dialog state, the
// delete mutation — lives here once; the desktop `<TableRow>` and the mobile
// card are then pure leaves fed the same values and the same handlers. The
// dialogs render identically in both, so they're handed back as one node.
export type IncomeRowView = {
  dateText: string;
  amountText: string;
  description: string | null;
  ownerLabel: string;
  categoryChip: string;
  regularity: Income["regularity"];
  regularityLabel: string;
  RegularityIcon: ComponentType<{ className?: string; "aria-label"?: string }>;
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  dialogs: React.ReactNode;
};

export function useIncomeRow(income: Income): IncomeRowView {
  const { t } = useTranslation(["income", "common"]);
  const [editOpen, setEditOpen] = useState(false);
  const [duplicateOpen, setDuplicateOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteMutation = useDeleteIncome();
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();

  const ownerLabel = ownershipLabel(
    income.ownership_type,
    income.sole_owner_user_id,
    members,
    currentUser,
  );

  function handleConfirmDelete() {
    deleteMutation.mutate(income.id, {
      onSuccess: () => setDeleteOpen(false),
    });
  }

  const seed: DuplicateSeed = {
    amount: income.amount,
    currency: income.currency,
    category: income.category,
    description: income.description,
    ownership_type: income.ownership_type,
    sole_owner_user_id: income.sole_owner_user_id,
    regularity: income.regularity,
  };

  const isRoutine = income.regularity === "routine";
  // Short row-chip label for the category (Salary / Business / Rental / ...)
  // — distinct from the longer dropdown options ("Business income") so the
  // cell/card stays compact.
  const categoryChip = t(`income:categories.${income.category}`);
  const regularityLabel = t(
    isRoutine ? "income:regularity.routineRowLabel" : "income:regularity.incidentalRowLabel",
  );

  const dialogs = (
    <>
      <EditIncomeDialog
        key={income.updated_at}
        open={editOpen}
        onOpenChange={setEditOpen}
        income={income}
      />

      {duplicateOpen && (
        <CreateIncomeDialog
          key={`dup-${income.id}-${duplicateOpen}`}
          open={duplicateOpen}
          onOpenChange={setDuplicateOpen}
          seed={seed}
          hideTrigger
        />
      )}

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("income:deleteTitle")}
        description={t("income:deleteDescription", {
          category: categoryChip,
          amount: formatCurrency(income.amount, income.currency),
          date: formatDate(income.date),
        })}
        confirmLabel={t("common:delete")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  );

  return {
    dateText: formatDate(income.date),
    amountText: formatCurrency(income.amount, income.currency),
    description: income.description || null,
    ownerLabel,
    categoryChip,
    regularity: income.regularity,
    regularityLabel,
    RegularityIcon: isRoutine ? Repeat : Sparkles,
    onEdit: () => setEditOpen(true),
    onDuplicate: () => setDuplicateOpen(true),
    onDelete: () => setDeleteOpen(true),
    dialogs,
  };
}
