import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { goldDescriptor } from "@/components/detail/descriptors/gold";

type Props = {
  investmentId: string;
  onBack: () => void;
};

// Gold detail page — now the generic `PositionDetailScreen` driven by the gold
// descriptor (ADR-0051, A4 — qty×price completion). The hand-written page body is
// gone; this thin wrapper keeps App.tsx's mount point + prop name stable.
export function GoldDetail({ investmentId, onBack }: Props) {
  return (
    <PositionDetailScreen descriptor={goldDescriptor} assetId={investmentId} onBack={onBack} />
  );
}
