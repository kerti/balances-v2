import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { receivableDescriptor } from "@/components/detail/descriptors/receivable";

type Props = {
  receivableId: string;
  onBack: () => void;
};

// Receivable detail page — now the generic `PositionDetailScreen` driven by the
// receivable descriptor (ADR-0051, A2). The hand-written page body is gone; this
// thin wrapper keeps App.tsx's mount point stable.
export function ReceivableDetail({ receivableId, onBack }: Props) {
  return (
    <PositionDetailScreen
      descriptor={receivableDescriptor}
      assetId={receivableId}
      onBack={onBack}
    />
  );
}
