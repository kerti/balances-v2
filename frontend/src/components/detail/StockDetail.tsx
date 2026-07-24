import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { stockDescriptor } from "@/components/detail/descriptors/stock";

type Props = {
  investmentId: string;
  onBack: () => void;
};

// Stock detail page — now the generic `PositionDetailScreen` driven by the stock
// descriptor (ADR-0051, A3 — the investment mechanism). The hand-written page
// body is gone; this thin wrapper keeps App.tsx's mount point + prop name stable.
export function StockDetail({ investmentId, onBack }: Props) {
  return (
    <PositionDetailScreen descriptor={stockDescriptor} assetId={investmentId} onBack={onBack} />
  );
}
