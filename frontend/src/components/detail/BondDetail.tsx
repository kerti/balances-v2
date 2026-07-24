import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bondDescriptor } from "@/components/detail/descriptors/bond";

type Props = {
  investmentId: string;
  onBack: () => void;
};

// Bond detail page — now the generic `PositionDetailScreen` driven by the bond
// descriptor (ADR-0051, A5 — accrued investments). The hand-written page body is
// gone; this thin wrapper keeps App.tsx's mount point + prop name stable.
export function BondDetail({ investmentId, onBack }: Props) {
  return (
    <PositionDetailScreen descriptor={bondDescriptor} assetId={investmentId} onBack={onBack} />
  );
}
