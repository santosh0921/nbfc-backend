import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:provider/provider.dart';
import 'package:provider/single_child_widget.dart';
import '../constants/employee_app_module.dart';
import '../providers/agency_provider.dart';
import '../providers/connectivity_provider.dart';
import '../providers/theme_provider.dart';
import '../storage/local_storage_service.dart';
import '../storage/mpin_service.dart';
import '../../features/auth/domain/repositories/auth_repository.dart';
import '../../features/auth/domain/usecases/login_usecase.dart';
import '../../features/auth/presentation/providers/auth_provider.dart';
import '../../features/cases/domain/repositories/cases_repository.dart';
import '../../features/cases/presentation/providers/cases_provider.dart';
import '../../features/collections/data/repositories/collections_repository_mock.dart';
import '../../features/collections/presentation/providers/collections_provider.dart';
import '../../features/dashboard/data/repositories/dashboard_repository_http.dart';
import '../../features/dashboard/presentation/providers/dashboard_provider.dart';
import '../../features/notifications/data/repositories/notifications_repository_http.dart';
import '../../features/notifications/presentation/providers/notifications_provider.dart';
import '../../features/visit_verification/data/repositories/visit_history_repository.dart';
import '../../features/visit_verification/presentation/providers/verification_draft_provider.dart';
import '../../features/visit_verification/presentation/providers/visit_history_provider.dart';
import '../../features/recovery/data/repositories/recovery_case_repository.dart';
import '../../features/recovery/data/repositories/visit_repository.dart';
import '../../features/recovery/data/repositories/payment_repository.dart';
import '../../features/recovery/data/repositories/collection_repository.dart';
import '../../features/recovery/presentation/providers/recovery_dashboard_provider.dart';
import '../../features/recovery/presentation/providers/recovery_cases_provider.dart';
import '../../features/recovery/presentation/providers/recovery_visit_provider.dart';
import '../../features/recovery/presentation/providers/recovery_report_provider.dart';
import 'service_locator.dart';

List<SingleChildWidget> appProviders({EmployeeAppModule module = EmployeeAppModule.verification}) {
  final sl = ServiceLocator.instance;

  return [
    ChangeNotifierProvider<ThemeProvider>(create: (_) => ThemeProvider(sl.get<LocalStorageService>())),
    ChangeNotifierProvider<AgencyProvider>(create: (_) => AgencyProvider(sl.get<LocalStorageService>())),
    ChangeNotifierProvider<ConnectivityProvider>(create: (_) => ConnectivityProvider(sl.get<Connectivity>())),
    ChangeNotifierProvider<AuthProvider>(
      create: (_) => AuthProvider(
        loginUseCase: sl.get<LoginUseCase>(),
        biometricLoginUseCase: sl.get<BiometricLoginUseCase>(),
        logoutUseCase: sl.get<LogoutUseCase>(),
        forgotPasswordUseCase: sl.get<ForgotPasswordUseCase>(),
        authRepository: sl.get<AuthRepository>(),
        localStorage: sl.get<LocalStorageService>(),
        mpinService: sl.get<MpinService>(),
        module: module,
      ),
    ),
    ChangeNotifierProvider<DashboardProvider>(
      create: (_) => DashboardProvider(sl.get<DashboardRepositoryHttp>()),
    ),
    Provider<CasesRepository>.value(value: sl.get<CasesRepository>()),
    ChangeNotifierProvider<CasesProvider>(
      create: (_) => CasesProvider(sl.get<CasesRepository>()),
    ),
    ChangeNotifierProvider<CollectionsProvider>(
      create: (_) => CollectionsProvider(sl.get<CollectionsRepositoryMock>()),
    ),
    ChangeNotifierProvider<NotificationsProvider>(
      create: (_) => NotificationsProvider(sl.get<NotificationsRepositoryHttp>()),
    ),
    ChangeNotifierProvider<VerificationDraftProvider>(
      create: (_) => VerificationDraftProvider(sl.get<VisitHistoryRepository>(), sl.get<CasesRepository>()),
    ),
    ChangeNotifierProvider<VisitHistoryProvider>(
      create: (_) => VisitHistoryProvider(sl.get<VisitHistoryRepository>()),
    ),
    Provider<RecoveryCaseRepository>.value(value: sl.get<RecoveryCaseRepository>()),
    Provider<VisitRepositoryMock>.value(value: sl.get<VisitRepositoryMock>()),
    Provider<PaymentRepositoryMock>.value(value: sl.get<PaymentRepositoryMock>()),
    Provider<CollectionRepositoryMock>.value(value: sl.get<CollectionRepositoryMock>()),
    ChangeNotifierProvider<RecoveryDashboardProvider>(
      create: (_) => RecoveryDashboardProvider(sl.get<CollectionRepositoryMock>()),
    ),
    ChangeNotifierProvider<RecoveryCasesProvider>(
      create: (_) => RecoveryCasesProvider(sl.get<RecoveryCaseRepository>()),
    ),
    ChangeNotifierProvider<RecoveryVisitProvider>(
      create: (_) => RecoveryVisitProvider(sl.get<VisitRepositoryMock>(), sl.get<PaymentRepositoryMock>()),
    ),
    ChangeNotifierProvider<RecoveryReportProvider>(create: (_) => RecoveryReportProvider()),
  ];
}
