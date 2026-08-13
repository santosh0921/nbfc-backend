import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';
import '../../features/auth/presentation/pages/forgot_password_page.dart';
import '../../features/auth/presentation/pages/login_page.dart';
import '../../features/auth/presentation/pages/mpin_setup_page.dart';
import '../../features/auth/presentation/pages/mpin_unlock_page.dart';
import '../../features/auth/presentation/pages/otp_verify_page.dart';
import '../../features/auth/presentation/pages/profile_setup_page.dart';
import '../../features/auth/presentation/pages/register_page.dart';
import '../../features/auth/presentation/providers/auth_provider.dart';
import '../../features/dashboard/presentation/pages/dashboard_home_page.dart';
import '../../features/dashboard/presentation/pages/dashboard_shell.dart';
import '../../features/cases/presentation/pages/cases_list_page.dart';
import '../../features/collections/presentation/pages/collections_page.dart';
import '../../features/notifications/presentation/pages/notifications_page.dart';
import '../../features/profile/presentation/pages/profile_page.dart';
import '../../features/recovery/presentation/pages/recovery_dashboard_page.dart';
import '../../features/recovery/presentation/pages/recovery_case_list_page.dart';
import '../../features/recovery/presentation/pages/recovery_case_detail_page.dart';
import '../../features/recovery/presentation/pages/recovery_visit_page.dart';
import '../../features/recovery/presentation/pages/recovery_report_page.dart';
import '../../features/recovery/presentation/pages/recovery_shell.dart';
import '../constants/employee_app_module.dart';
import '../providers/agency_provider.dart';
import 'app_page_transition.dart';
import 'route_names.dart';

class AppRouter {
  AppRouter._();

  static const _publicPaths = [RouteNames.loginPath, RouteNames.registerPath, RouteNames.forgotPasswordPath];

  static bool _isPublicPath(String path) => _publicPaths.any((p) => path.startsWith(p));

  static GoRouter build(
    AuthProvider authProvider,
    AgencyProvider agencyProvider, {
    EmployeeAppModule module = EmployeeAppModule.verification,
  }) {
    final homePath = module == EmployeeAppModule.recovery ? RouteNames.recoveryDashboardPath : RouteNames.dashboardPath;
    return GoRouter(
      initialLocation: RouteNames.loginPath,
      refreshListenable: Listenable.merge([authProvider, agencyProvider]),
      redirect: (context, state) {
        final path = state.matchedLocation;

        // Strict priority chain — each stage owns exactly one redirect
        // target, so re-evaluating after a redirect always resolves to
        // `null` (stay) instead of bouncing to a different stage's target.
        final isAuthenticated = authProvider.status == AuthStatus.authenticated;
        if (!isAuthenticated) {
          return _isPublicPath(path) ? null : RouteNames.loginPath;
        }

        // A restored JWT session behind a local MPIN app-lock takes
        // priority over everything else post-auth — nothing else (profile
        // setup, onboarding, the dashboard) should be reachable until it's
        // cleared.
        if (authProvider.needsMpinUnlock) {
          return path == RouteNames.mpinUnlockPath ? null : RouteNames.mpinUnlockPath;
        }

        // A brand-new login on this device walks through the one-time
        // mock-OTP + MPIN-setup sequence before anything else.
        if (authProvider.needsFirstTimeOnboarding) {
          final onOnboardingPath = path == RouteNames.otpVerifyPath || path == RouteNames.mpinSetupPath;
          return onOnboardingPath ? null : RouteNames.otpVerifyPath;
        }

        if (authProvider.needsProfileSetup) {
          return path == RouteNames.profileSetupPath ? null : RouteNames.profileSetupPath;
        }

        final isPreAuthPath = path == RouteNames.profileSetupPath ||
            path == RouteNames.otpVerifyPath ||
            path == RouteNames.mpinSetupPath ||
            path == RouteNames.mpinUnlockPath ||
            _isPublicPath(path);
        if (isPreAuthPath) return homePath;

        return null;
      },
      routes: [
        GoRoute(
          path: RouteNames.loginPath,
          name: RouteNames.login,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const LoginPage()),
        ),
        GoRoute(
          path: RouteNames.registerPath,
          name: RouteNames.register,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const RegisterPage()),
        ),
        GoRoute(
          path: RouteNames.forgotPasswordPath,
          name: RouteNames.forgotPassword,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const ForgotPasswordPage()),
        ),
        GoRoute(
          path: RouteNames.profileSetupPath,
          name: RouteNames.profileSetup,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const ProfileSetupPage()),
        ),
        GoRoute(
          path: RouteNames.otpVerifyPath,
          name: RouteNames.otpVerify,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const OtpVerifyPage()),
        ),
        GoRoute(
          path: RouteNames.mpinSetupPath,
          name: RouteNames.mpinSetup,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const MpinSetupPage()),
        ),
        GoRoute(
          path: RouteNames.mpinUnlockPath,
          name: RouteNames.mpinUnlock,
          pageBuilder: (context, state) =>
              buildPageWithTransition(context: context, state: state, child: const MpinUnlockPage()),
        ),
        StatefulShellRoute.indexedStack(
          builder: (context, state, navigationShell) => DashboardShell(navigationShell: navigationShell),
          branches: [
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.dashboardPath,
                name: RouteNames.dashboard,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const DashboardHomePage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.casesPath,
                name: RouteNames.cases,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const CasesListPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.collectionsPath,
                name: RouteNames.collections,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const CollectionsPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.notificationsPath,
                name: RouteNames.notifications,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const NotificationsPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.profilePath,
                name: RouteNames.profile,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const ProfilePage()),
              ),
            ]),
          ],
        ),
        StatefulShellRoute.indexedStack(
          builder: (context, state, navigationShell) => RecoveryShell(navigationShell: navigationShell),
          branches: [
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.recoveryDashboardPath,
                name: RouteNames.recoveryDashboard,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const RecoveryDashboardPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.recoveryCasesPath,
                name: RouteNames.recoveryCases,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const RecoveryCaseListPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.recoveryAlertsPath,
                name: RouteNames.recoveryAlerts,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const NotificationsPage()),
              ),
            ]),
            StatefulShellBranch(routes: [
              GoRoute(
                path: RouteNames.recoveryProfilePath,
                name: RouteNames.recoveryProfile,
                pageBuilder: (context, state) =>
                    buildPageWithTransition(context: context, state: state, child: const ProfilePage()),
              ),
            ]),
          ],
        ),
        GoRoute(
          path: RouteNames.recoveryCaseDetailPath,
          name: RouteNames.recoveryCaseDetail,
          pageBuilder: (context, state) => buildPageWithTransition(
            context: context,
            state: state,
            child: RecoveryCaseDetailPage(caseId: state.pathParameters['id']!),
          ),
        ),
        GoRoute(
          path: RouteNames.recoveryVisitPath,
          name: RouteNames.recoveryVisit,
          pageBuilder: (context, state) => buildPageWithTransition(
            context: context,
            state: state,
            child: RecoveryVisitPage(caseId: state.pathParameters['id']!),
          ),
        ),
        GoRoute(
          path: RouteNames.recoveryReportPath,
          name: RouteNames.recoveryReport,
          pageBuilder: (context, state) => buildPageWithTransition(
            context: context,
            state: state,
            child: RecoveryReportPage(caseId: state.pathParameters['id']!),
          ),
        ),
      ],
    );
  }
}
