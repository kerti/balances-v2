import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { vehicleDescriptor } from "@/components/detail/descriptors/vehicle";

type Props = {
  assetId: string;
  onBack: () => void;
};

// Vehicle detail page — now the generic `PositionDetailScreen` driven by the
// vehicle descriptor (ADR-0051, A2). The hand-written page body is gone; this
// thin wrapper keeps App.tsx's mount point stable.
export function VehicleDetail({ assetId, onBack }: Props) {
  return <PositionDetailScreen descriptor={vehicleDescriptor} assetId={assetId} onBack={onBack} />;
}
