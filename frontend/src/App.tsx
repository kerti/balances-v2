import type { ReactNode } from "react";
import {
  createBrowserRouter,
  Navigate,
  RouterProvider,
  useNavigate,
  useParams,
  type NavigateFunction,
} from "react-router";
import { useSession } from "@/hooks/useSession";
import { routes } from "@/lib/routes";
import { SignInScreen } from "@/components/SignInScreen";
import { OnboardingScreen } from "@/components/OnboardingScreen";
import { InviteAcceptScreen } from "@/components/InviteAcceptScreen";
import { AppShell } from "@/components/AppShell";
import { RouteErrorBoundary } from "@/components/RouteErrorBoundary";
import { DashboardScreen } from "@/components/DashboardScreen";
import { AssetsHome } from "@/components/AssetsHome";
import { BankAccountsScreen } from "@/components/BankAccountsScreen";
import { BankAccountDetail } from "@/components/BankAccountDetail";
import { PropertiesScreen } from "@/components/PropertiesScreen";
import { PropertyDetail } from "@/components/PropertyDetail";
import { VehiclesScreen } from "@/components/VehiclesScreen";
import { VehicleDetail } from "@/components/VehicleDetail";
import { LiabilitiesHome } from "@/components/LiabilitiesHome";
import { LiabilitiesScreen } from "@/components/LiabilitiesScreen";
import { LiabilityDetail } from "@/components/LiabilityDetail";
import { ReceivablesScreen } from "@/components/ReceivablesScreen";
import { ReceivableDetail } from "@/components/ReceivableDetail";
import { InvestmentsHome } from "@/components/InvestmentsHome";
import { StocksScreen } from "@/components/StocksScreen";
import { StockDetail } from "@/components/StockDetail";
import { MutualFundsScreen } from "@/components/MutualFundsScreen";
import { MutualFundDetail } from "@/components/MutualFundDetail";
import { BondsScreen } from "@/components/BondsScreen";
import { BondDetail } from "@/components/BondDetail";
import { TimeDepositsScreen } from "@/components/TimeDepositsScreen";
import { TimeDepositDetail } from "@/components/TimeDepositDetail";
import { GoldsScreen } from "@/components/GoldsScreen";
import { GoldDetail } from "@/components/GoldDetail";
import { IncomeScreen } from "@/components/IncomeScreen";
import { TagsScreen } from "@/components/TagsScreen";
import { SettingsScreen } from "@/components/SettingsScreen";

// The list screens and detail pages predate the router: they take an
// `onSelect(id)` / `onBack()` callback and the entity id as a prop, with no
// router awareness. These two thin wrappers bridge that contract to the router
// so the ~20 screen/detail components stay untouched — the router lives only
// here. ListRoute hands the screen a navigate fn; DetailRoute also pulls the
// `:id` path param.
function ListRoute({
  render,
}: {
  render: (nav: NavigateFunction) => ReactNode;
}) {
  return <>{render(useNavigate())}</>;
}

function DetailRoute({
  render,
}: {
  render: (id: string, nav: NavigateFunction) => ReactNode;
}) {
  const { id } = useParams();
  return <>{render(id!, useNavigate())}</>;
}

const router = createBrowserRouter([
  {
    element: <AppShell />,
    // Any error thrown while rendering a route subtree (notably a lazy chart
    // chunk that failed twice — past the lazyWithReload one-shot) surfaces here
    // instead of React Router's raw developer dump (#191).
    errorElement: <RouteErrorBoundary />,
    children: [
      { index: true, element: <DashboardScreen /> },

      // Assets
      { path: "assets", element: <AssetsHome /> },
      {
        path: "assets/bank-accounts",
        element: (
          <ListRoute
            render={(nav) => (
              <BankAccountsScreen
                onSelect={(id) => nav(routes.bankAccount(id))}
              />
            )}
          />
        ),
      },
      {
        path: "assets/bank-accounts/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <BankAccountDetail
                assetId={id}
                onBack={() => nav(routes.bankAccounts)}
              />
            )}
          />
        ),
      },
      {
        path: "assets/properties",
        element: (
          <ListRoute
            render={(nav) => (
              <PropertiesScreen onSelect={(id) => nav(routes.property(id))} />
            )}
          />
        ),
      },
      {
        path: "assets/properties/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <PropertyDetail
                assetId={id}
                onBack={() => nav(routes.properties)}
              />
            )}
          />
        ),
      },
      {
        path: "assets/vehicles",
        element: (
          <ListRoute
            render={(nav) => (
              <VehiclesScreen onSelect={(id) => nav(routes.vehicle(id))} />
            )}
          />
        ),
      },
      {
        path: "assets/vehicles/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <VehicleDetail assetId={id} onBack={() => nav(routes.vehicles)} />
            )}
          />
        ),
      },

      // Liabilities
      { path: "liabilities", element: <LiabilitiesHome /> },
      {
        path: "liabilities/personal",
        element: (
          <ListRoute
            render={(nav) => (
              <LiabilitiesScreen
                subtype="personal"
                onSelect={(id) => nav(routes.liability("personal", id))}
              />
            )}
          />
        ),
      },
      {
        path: "liabilities/personal/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <LiabilityDetail
                liabilityId={id}
                onBack={() => nav(routes.liabilitiesPersonal)}
              />
            )}
          />
        ),
      },
      {
        path: "liabilities/institutional",
        element: (
          <ListRoute
            render={(nav) => (
              <LiabilitiesScreen
                subtype="institutional"
                onSelect={(id) => nav(routes.liability("institutional", id))}
              />
            )}
          />
        ),
      },
      {
        path: "liabilities/institutional/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <LiabilityDetail
                liabilityId={id}
                onBack={() => nav(routes.liabilitiesInstitutional)}
              />
            )}
          />
        ),
      },

      // Receivables (flat)
      {
        path: "receivables",
        element: (
          <ListRoute
            render={(nav) => (
              <ReceivablesScreen
                onSelect={(id) => nav(routes.receivable(id))}
              />
            )}
          />
        ),
      },
      {
        path: "receivables/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <ReceivableDetail
                receivableId={id}
                onBack={() => nav(routes.receivables)}
              />
            )}
          />
        ),
      },

      // Investments
      { path: "investments", element: <InvestmentsHome /> },
      {
        path: "investments/stocks",
        element: (
          <ListRoute
            render={(nav) => (
              <StocksScreen onSelect={(id) => nav(routes.stock(id))} />
            )}
          />
        ),
      },
      {
        path: "investments/stocks/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <StockDetail
                investmentId={id}
                onBack={() => nav(routes.stocks)}
              />
            )}
          />
        ),
      },
      {
        path: "investments/mutual-funds",
        element: (
          <ListRoute
            render={(nav) => (
              <MutualFundsScreen
                onSelect={(id) => nav(routes.mutualFund(id))}
              />
            )}
          />
        ),
      },
      {
        path: "investments/mutual-funds/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <MutualFundDetail
                investmentId={id}
                onBack={() => nav(routes.mutualFunds)}
              />
            )}
          />
        ),
      },
      {
        path: "investments/bonds",
        element: (
          <ListRoute
            render={(nav) => (
              <BondsScreen onSelect={(id) => nav(routes.bond(id))} />
            )}
          />
        ),
      },
      {
        path: "investments/bonds/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <BondDetail investmentId={id} onBack={() => nav(routes.bonds)} />
            )}
          />
        ),
      },
      {
        path: "investments/time-deposits",
        element: (
          <ListRoute
            render={(nav) => (
              <TimeDepositsScreen
                onSelect={(id) => nav(routes.timeDeposit(id))}
              />
            )}
          />
        ),
      },
      {
        path: "investments/time-deposits/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <TimeDepositDetail
                investmentId={id}
                onBack={() => nav(routes.timeDeposits)}
                onSelectTimeDeposit={(tid) => nav(routes.timeDeposit(tid))}
              />
            )}
          />
        ),
      },
      {
        path: "investments/gold",
        element: (
          <ListRoute
            render={(nav) => (
              <GoldsScreen onSelect={(id) => nav(routes.goldItem(id))} />
            )}
          />
        ),
      },
      {
        path: "investments/gold/:id",
        element: (
          <DetailRoute
            render={(id, nav) => (
              <GoldDetail investmentId={id} onBack={() => nav(routes.gold)} />
            )}
          />
        ),
      },

      // Income (flat flow event)
      { path: "income", element: <IncomeScreen /> },

      // Tags breakdown report (flat, like Income — no detail pages).
      { path: "tags", element: <TagsScreen /> },

      { path: "settings", element: <SettingsScreen /> },

      // Unknown path → dashboard.
      { path: "*", element: <Navigate to={routes.dashboard} replace /> },
    ],
  },
]);

function App() {
  const { data: user, isPending } = useSession();

  if (isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center text-muted-foreground">
        Loading…
      </div>
    );
  }

  if (!user) {
    // The post-auth onboarding gate (ADR-0038) lives before the authed router:
    // its visitor holds a handshake cookie but has no session, so useSession
    // returns null. The handshake — not the URL — is the real credential; an
    // invalid one makes /onboarding/options answer 401, which OnboardingScreen
    // surfaces as a "sign in again" prompt.
    if (window.location.pathname === routes.onboarding) {
      return <OnboardingScreen />;
    }
    // Local-invite accept (ADR-0039/#281): the invitee holds only the URL token,
    // no session or handshake — the screen resolves it and sets a password.
    if (window.location.pathname === routes.accept) {
      return <InviteAcceptScreen />;
    }
    return <SignInScreen />;
  }

  return <RouterProvider router={router} />;
}

export default App;
