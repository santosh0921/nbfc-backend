import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../providers/auth_provider.dart';
import 'app_shell.dart';
import '../../features/apply/quick_apply_screen.dart';
import '../../features/auth/biometric_auth_page.dart';
import '../../features/auth/create_mpin_page.dart';
import '../../features/auth/login_page.dart';
import '../../features/auth/mpin_unlock_page.dart';
import '../../features/auth/otp_verify_page.dart';
import '../../features/auth/registration_page.dart';
import '../../features/compliance/compliance_screen.dart';
import '../../features/credit_score/credit_score_screen.dart';
import '../../features/home/home_screen.dart';
import '../../features/loans/explore_loans_screen.dart';
import '../../features/loans/loan_detail_screen.dart';
import '../../features/marketplace/marketplace_category_screen.dart';
import '../../features/marketplace/marketplace_screen.dart';
import '../../models/loan_product.dart';
import '../../models/offer_banner.dart';
import '../../features/menu/menu_screen.dart';
import '../../features/profile/profile_screen.dart';
import '../../features/notifications/notifications_screen.dart';
import '../../features/my_loans/my_loans_screen.dart';
import '../../features/payments/payment_screen.dart';
import '../../presentation/splash/splash_screen.dart';
import '../../features/support/support_screen.dart';
import '../../features/documents/documents_screen.dart';
import '../../features/security/security_screen.dart';
import '../../features/language/language_screen.dart';
import '../../features/track_application/track_application_screen.dart';
import '../../features/permissions/app_permissions_screen.dart';
import '../../features/loans/emi_calculator_screen.dart';
import '../../features/loans/emi_schedule_screen.dart';
import '../../employee_app/core/constants/employee_app_module.dart';
import '../../features/employee_gate/employee_gate_screen.dart';
import '../../features/employee_gate/employee_role_selection_screen.dart';
import '../../features/onboarding/onboarding_screen.dart';
import '../../features/transactions/transaction_history_screen.dart';

final GlobalKey<NavigatorState> _rootNavigatorKey = GlobalKey<NavigatorState>(debugLabel: 'root');

const _publicPaths = [
  '/splash',
  '/onboarding',
  '/login',
  '/otp',
  '/register',
  '/create-mpin',
  '/mpin-unlock',
  '/biometric',
  '/employee',
];

class AppRouter {
  AppRouter._();

  static final GoRouter router = GoRouter(
    navigatorKey: _rootNavigatorKey,
    initialLocation: '/splash',
    refreshListenable: AuthProvider.instance,
    redirect: (context, state) {
      final path = state.matchedLocation;
      final isPublic = _publicPaths.any((p) => path.startsWith(p));
      if (!AuthProvider.instance.hasSession && !isPublic) {
        return '/login';
      }
      if (AuthProvider.instance.hasSession && path == '/login') {
        return '/';
      }
      return null;
    },
    routes: [
      GoRoute(
        path: '/splash',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const SplashScreen(),
      ),
      GoRoute(
        path: '/onboarding',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const OnboardingScreen(),
      ),
      GoRoute(
        path: '/login',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const LoginPage(),
      ),
      GoRoute(
        path: '/employee',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const EmployeeRoleSelectionScreen(),
      ),
      GoRoute(
        path: '/employee/verification',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const EmployeeGateScreen(module: EmployeeAppModule.verification),
      ),
      GoRoute(
        path: '/employee/recovery',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const EmployeeGateScreen(module: EmployeeAppModule.recovery),
      ),
      GoRoute(
        path: '/otp',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) {
          final extra = state.extra;
          if (extra is ({String phone, String? devOtp})) {
            return OtpVerifyPage(phoneNumber: extra.phone, devOtp: extra.devOtp);
          }
          return OtpVerifyPage(phoneNumber: extra as String?);
        },
      ),
      GoRoute(
        path: '/register',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const RegistrationPage(),
      ),
      GoRoute(
        path: '/create-mpin',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const CreateMpinPage(),
      ),
      GoRoute(
        path: '/mpin-unlock',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const MpinUnlockPage(),
      ),
      GoRoute(
        path: '/biometric',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const BiometricAuthPage(),
      ),
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) => AppShell(navigationShell: navigationShell),
        branches: [
          StatefulShellBranch(routes: [
            GoRoute(path: '/', builder: (context, state) => const HomeScreen()),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/loans', builder: (context, state) => const ExploreLoansScreen()),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/marketplace', builder: (context, state) => const MarketplaceScreen()),
          ]),
          StatefulShellBranch(routes: [
            GoRoute(path: '/profile', builder: (context, state) => const ProfileScreen()),
          ]),
        ],
      ),
      GoRoute(
        path: '/menu',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const MenuScreen(),
      ),
      GoRoute(
        path: '/loans/:id',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => LoanDetailScreen(loanId: state.pathParameters['id']!),
      ),
      GoRoute(
        path: '/quick-apply',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => QuickApplyScreen(product: state.extra as LoanProduct?),
      ),
      GoRoute(
        path: '/my-loans',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) {
          final extra = state.extra;
          if (extra is ({int loanId, String? letterType})) {
            return MyLoansScreen(initialLoanId: extra.loanId, initialLetterType: extra.letterType);
          }
          return const MyLoansScreen();
        },
      ),
      GoRoute(
        path: '/payments',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const PaymentScreen(),
      ),
      GoRoute(
        path: '/notifications',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const NotificationsScreen(),
      ),
      GoRoute(
        path: '/support',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const SupportScreen(),
      ),
      GoRoute(
        path: '/compliance',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const ComplianceScreen(),
      ),
      GoRoute(
        path: '/marketplace-item',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => MarketplaceCategoryScreen(item: state.extra as MarketplaceItem),
      ),
      GoRoute(
        path: '/credit-score',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const CreditScoreScreen(),
      ),
      GoRoute(
        path: '/documents',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const DocumentsScreen(),
      ),
      GoRoute(
        path: '/security',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const SecurityScreen(),
      ),
      GoRoute(
        path: '/language',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const LanguageScreen(),
      ),
      GoRoute(
        path: '/track-application',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const TrackApplicationScreen(),
      ),
      GoRoute(
        path: '/permissions',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const AppPermissionsScreen(),
      ),
      GoRoute(
        path: '/emi-calculator',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const EmiCalculatorScreen(),
      ),
      GoRoute(
        path: '/emi-schedule',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => EmiScheduleScreen(loanId: state.extra as String?),
      ),
      GoRoute(
        path: '/transactions',
        parentNavigatorKey: _rootNavigatorKey,
        builder: (context, state) => const TransactionHistoryScreen(),
      ),
    ],
  );
}
