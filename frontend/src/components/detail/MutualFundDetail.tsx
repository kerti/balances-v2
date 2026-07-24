import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { mutualFundDescriptor } from "@/components/detail/descriptors/mutualFund";

type Props = {
  investmentId: string;
  onBack: () => void;
};

// Mutual-fund detail page — now the generic `PositionDetailScreen` driven by the
// mutual-fund descriptor (ADR-0051, A4 — qty×price completion). The hand-written
// page body is gone; this thin wrapper keeps App.tsx's mount point + prop name
// stable.
export function MutualFundDetail({ investmentId, onBack }: Props) {
  return (
    <PositionDetailScreen
      descriptor={mutualFundDescriptor}
      assetId={investmentId}
      onBack={onBack}
    />
  );
}
