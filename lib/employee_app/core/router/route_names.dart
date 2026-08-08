class RouteNames {
  RouteNames._();

  // Pre-auth
  static const String agencySelect = 'agencySelect';
  static const String agencySelectPath = '/agency-select';

  // Auth
  static const String login = 'login';
  static const String loginPath = '/login';

  static const String forgotPassword = 'forgotPassword';
  static const String forgotPasswordPath = '/forgot-password';

  static const String profileSetup = 'profileSetup';
  static const String profileSetupPath = '/profile-setup';

  // First-time onboarding (mock OTP + MPIN setup), and MPIN quick-unlock
  // for an already-restored JWT session.
  static const String otpVerify = 'otpVerify';
  static const String otpVerifyPath = '/otp-verify';

  static const String mpinSetup = 'mpinSetup';
  static const String mpinSetupPath = '/mpin-setup';

  static const String mpinUnlock = 'mpinUnlock';
  static const String mpinUnlockPath = '/mpin-unlock';

  // Dashboard shell
  static const String dashboard = 'dashboard';
  static const String dashboardPath = '/dashboard';

  static const String cases = 'cases';
  static const String casesPath = '/dashboard/cases';

  static const String collections = 'collections';
  static const String collectionsPath = '/dashboard/collections';

  static const String notifications = 'notifications';
  static const String notificationsPath = '/dashboard/notifications';

  static const String profile = 'profile';
  static const String profilePath = '/dashboard/profile';

  // Recovery shell
  static const String recoveryDashboard = 'recoveryDashboard';
  static const String recoveryDashboardPath = '/recovery';

  static const String recoveryCases = 'recoveryCases';
  static const String recoveryCasesPath = '/recovery/cases';

  static const String recoveryAlerts = 'recoveryAlerts';
  static const String recoveryAlertsPath = '/recovery/alerts';

  static const String recoveryProfile = 'recoveryProfile';
  static const String recoveryProfilePath = '/recovery/profile';

  static const String recoveryCaseDetail = 'recoveryCaseDetail';
  static const String recoveryCaseDetailPath = '/recovery/cases/:id';

  static const String recoveryVisit = 'recoveryVisit';
  static const String recoveryVisitPath = '/recovery/cases/:id/visit';

  static const String recoveryReport = 'recoveryReport';
  static const String recoveryReportPath = '/recovery/cases/:id/report';
}
