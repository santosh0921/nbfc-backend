# NBFC Premium — Frontend Prototype

A frontend-only, mock-data-driven Flutter app for a modern Indian NBFC (inspired
by, not a copy of, apps like L&T Finance). No backend, no APIs, no Firebase,
no auth — everything is powered by static/mock data in `lib/mock/mock_data.dart`.

## Run it

```bash
flutter pub get
flutter run
```

Flutter SDK was not available in the environment this was authored in, so the
code has not been compiled/run yet. If `flutter analyze` surfaces issues,
they're most likely minor (import ordering, a missed const, a package version
pin) — fix and iterate.

## What's built

- **Theme**: full Material 3 light + dark theme, blue/gold NBFC palette,
  Manrope/Inter type scale (`lib/core/theme`).
- **Navigation**: `go_router` with a `StatefulShellRoute` bottom-nav shell
  (Home / Loans / Marketplace / Profile) + a floating Quick Apply FAB, plus
  pushed routes for loan detail, my loans, notifications, and stubs for
  payments/support/compliance/credit-score (`lib/core/router`).
- **Home dashboard**: hero banner carousel, pre-approved offer card, credit
  score card, EMI reminder, quick actions, explore-loans grid, marketplace
  rail, recent transactions, financial tips (`lib/features/home`).
- **Loans**: explore grid across all 14 product categories + a full loan
  detail page (hero, rate/APR/tenure stats, benefits, eligibility, documents,
  EMI calculator, FAQ accordion, related products) (`lib/features/loans`).
- **My Loans**: active loan cards with progress, outstanding, next EMI.
- **Notifications, Profile** (with dark-mode toggle wired to a `Provider`).
- **Stub screens** (`ComingSoonScaffold`) for Payments, Support, Compliance,
  Credit Score detail, Quick Apply flow — reachable via router so the app is
  navigable end-to-end; these are the natural next screens to flesh out.

## Project structure

```
lib/
  core/
    theme/        color tokens, text styles, spacing, ThemeData
    router/        go_router config + bottom-nav shell
    providers/     ThemeProvider (light/dark)
    widgets/       shared building blocks (PremiumCard, GradientContainer, ...)
  features/
    home/
    loans/
    marketplace/
    my_loans/
    notifications/
    profile/
  models/          plain Dart data classes
  mock/            MockData — the single source of realistic Indian mock data
```

## Next screens to build out

Application flow (multi-step apply), Payments (EMI/AutoPay/mandate/history/
receipt with mock success & failure), Support (FAQs/complaints/branch
locator), and the RBI compliance informational screens (KFS, APR disclosure,
Fair Practices Code, grievance officer, etc.) are scaffolded as routes but
not yet designed — follow the same `PremiumCard` / gradient-hero / section
patterns used in Home and Loan Detail to keep visual consistency.
