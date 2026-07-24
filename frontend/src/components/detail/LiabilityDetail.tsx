import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { liabilityDescriptor } from "@/components/detail/descriptors/liability";

type Props = {
  liabilityId: string;
  onBack: () => void;
};

// Liability detail page — now the generic `PositionDetailScreen` driven by the
// liability descriptor (ADR-0051, A2). The hand-written page body is gone; this
// thin wrapper keeps App.tsx's mount points (personal + institutional) stable.
export function LiabilityDetail({ liabilityId, onBack }: Props) {
  return (
    <PositionDetailScreen descriptor={liabilityDescriptor} assetId={liabilityId} onBack={onBack} />
  );
}
