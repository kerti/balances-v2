import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { propertyDescriptor } from "@/components/detail/descriptors/property";

type Props = {
  assetId: string;
  onBack: () => void;
};

// Property detail page — now the generic `PositionDetailScreen` driven by the
// property descriptor (ADR-0051, A2). The hand-written page body is gone; this
// thin wrapper keeps App.tsx's mount point stable.
export function PropertyDetail({ assetId, onBack }: Props) {
  return <PositionDetailScreen descriptor={propertyDescriptor} assetId={assetId} onBack={onBack} />;
}
