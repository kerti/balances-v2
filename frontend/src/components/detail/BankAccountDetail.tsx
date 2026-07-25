import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bankAccountDescriptor } from "@/components/detail/descriptors/bankAccount";

type Props = {
  assetId: string;
  onBack: () => void;
};

// Bank account detail page — now the generic `PositionDetailScreen` driven by
// the bank-account descriptor (ADR-0051, A1 linchpin). The hand-written page
// body is gone; this thin wrapper keeps App.tsx's mount point stable.
export function BankAccountDetail({ assetId, onBack }: Props) {
  return (
    <PositionDetailScreen descriptor={bankAccountDescriptor} assetId={assetId} onBack={onBack} />
  );
}
