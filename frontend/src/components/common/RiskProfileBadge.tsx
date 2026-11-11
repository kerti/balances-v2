import { useTranslation } from "react-i18next";
import { Shield, ShieldAlert, ShieldHalf } from "lucide-react";
import type { RiskProfile } from "@/api/types";
import { InfoHint } from "@/components/common/InfoHint";

// RiskProfileBadge — icon + single-letter pip rendered next to a position's
// display name on every investment list screen.
//
// Monochrome (ADR-0054 amendment 2026-11-09): risk is not good or bad to own,
// so the shield shape, the L/M/H letter and the accessible label carry the
// level, never colour.

type Props = {
  profile: RiskProfile;
  /** Compact = icon-only; otherwise icon + uppercase letter (L/M/H). */
  compact?: boolean;
};

const META: Record<
  RiskProfile,
  {
    Icon: typeof Shield;
    labelKey: string;
    letterKey: string;
  }
> = {
  low: {
    Icon: Shield,
    labelKey: "riskProfile.badgeLow",
    letterKey: "riskProfile.badgeLetterLow",
  },
  medium: {
    Icon: ShieldHalf,
    labelKey: "riskProfile.badgeMedium",
    letterKey: "riskProfile.badgeLetterMedium",
  },
  high: {
    Icon: ShieldAlert,
    labelKey: "riskProfile.badgeHigh",
    letterKey: "riskProfile.badgeLetterHigh",
  },
};

export function RiskProfileBadge({ profile, compact = false }: Props) {
  const { t } = useTranslation("investments");
  const { Icon, labelKey, letterKey } = META[profile];
  const label = t(labelKey);
  const letter = t(letterKey);
  return (
    <span
      className="inline-flex items-center text-muted-foreground"
      data-testid={`risk-profile-${profile}`}
    >
      <InfoHint
        label={label}
        testId={`risk-profile-hint-${profile}`}
        className="gap-1 rounded-sm px-1"
        trigger={
          <>
            <Icon className="size-3.5" aria-hidden="true" />
            {!compact && <span className="text-xs font-semibold">{letter}</span>}
          </>
        }
      >
        {label}
      </InfoHint>
    </span>
  );
}
