import { useTranslation } from "react-i18next";
import { Shield, ShieldAlert, ShieldHalf } from "lucide-react";
import type { RiskProfile } from "@/api/types";
import { InfoHint } from "@/components/common/InfoHint";

// RiskProfileBadge — icon + single-letter pip rendered next to a position's
// display name on every investment list screen.
//
// Shield progression: ShieldHalf (medium, neutral amber) sits between
// ShieldCheck-like solidity at the safe end and ShieldAlert at the warning
// end. The colour choice tracks the dark-mode Stone+Cyan theme: emerald for
// low (calming), amber for medium (neutral attention), rose for high
// (warning) — none of these overlap with status badges.

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
    colour: string;
  }
> = {
  low: {
    Icon: Shield,
    labelKey: "riskProfile.badgeLow",
    letterKey: "riskProfile.badgeLetterLow",
    colour: "text-emerald-500",
  },
  medium: {
    Icon: ShieldHalf,
    labelKey: "riskProfile.badgeMedium",
    letterKey: "riskProfile.badgeLetterMedium",
    colour: "text-amber-500",
  },
  high: {
    Icon: ShieldAlert,
    labelKey: "riskProfile.badgeHigh",
    letterKey: "riskProfile.badgeLetterHigh",
    colour: "text-rose-500",
  },
};

export function RiskProfileBadge({ profile, compact = false }: Props) {
  const { t } = useTranslation("investments");
  const { Icon, labelKey, letterKey, colour } = META[profile];
  const label = t(labelKey);
  const letter = t(letterKey);
  return (
    <span className={`inline-flex items-center ${colour}`} data-testid={`risk-profile-${profile}`}>
      <InfoHint
        label={label}
        testId={`risk-profile-hint-${profile}`}
        className={`${colour} gap-1 rounded-sm px-1`}
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
