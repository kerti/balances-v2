import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { timeDepositDescriptor } from "@/components/detail/descriptors/timeDeposit";

type Props = {
  investmentId: string;
  onBack: () => void;
};

// TimeDeposit detail page — now the generic `PositionDetailScreen` driven by the
// time-deposit descriptor (ADR-0051, A5 — the accrued outlier). The hand-written
// page body is gone; this thin wrapper keeps App.tsx's mount point + prop name
// stable. The rollover-chain navigation the old page took as an `onSelectTimeDeposit`
// prop now lives inside the descriptor's context (via `useNavigate`), so the
// wrapper's prop shape matches every other detail page.
export function TimeDepositDetail({ investmentId, onBack }: Props) {
  return (
    <PositionDetailScreen
      descriptor={timeDepositDescriptor}
      assetId={investmentId}
      onBack={onBack}
    />
  );
}
