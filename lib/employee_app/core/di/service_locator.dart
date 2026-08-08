import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:hive/hive.dart';
import '../network/api_client.dart';
import '../storage/local_storage_service.dart';
import '../storage/secure_storage_service.dart';
import '../../features/auth/data/datasources/auth_remote_datasource.dart';
import '../../features/auth/data/repositories/auth_repository_impl.dart';
import '../../features/auth/domain/repositories/auth_repository.dart';
import '../../features/auth/domain/usecases/login_usecase.dart';
import '../../features/dashboard/data/repositories/dashboard_repository_mock.dart';
import '../../features/cases/data/repositories/cases_repository_http.dart';
import '../../features/cases/domain/repositories/cases_repository.dart';
import '../../features/collections/data/repositories/collections_repository_mock.dart';
import '../../features/notifications/data/repositories/notifications_repository_http.dart';
import '../../features/visit_verification/data/repositories/visit_history_repository.dart';
import '../../features/recovery/data/repositories/recovery_case_repository.dart';
import '../../features/recovery/data/repositories/recovery_case_repository_http.dart';
import '../../features/recovery/data/repositories/visit_repository.dart';
import '../../features/recovery/data/repositories/payment_repository.dart';
import '../../features/recovery/data/repositories/collection_repository.dart';

/// Manual service locator — mirrors the customer app's DI pattern (no
/// get_it). Wires datasources -> repositories -> usecases; `providers.dart`
/// turns this into the widget-tree `MultiProvider` list.
class ServiceLocator {
  ServiceLocator._();
  static final ServiceLocator instance = ServiceLocator._();

  final Map<Type, Object> _instances = {};

  T get<T>() => _instances[T] as T;

  Future<void> init() async {
    // Core
    _instances[LocalStorageService] = LocalStorageService();
    _instances[SecureStorageService] = SecureStorageService();
    _instances[Connectivity] = Connectivity();
    _instances[ApiClient] = ApiClient(get<SecureStorageService>());

    // Auth
    _instances[AuthRemoteDataSource] = AuthRemoteDataSource(get<ApiClient>());
    _instances[AuthRepository] = AuthRepositoryImpl(get<AuthRemoteDataSource>(), get<SecureStorageService>());
    _instances[LoginUseCase] = LoginUseCase(get<AuthRepository>());
    _instances[BiometricLoginUseCase] = BiometricLoginUseCase(get<AuthRepository>());
    _instances[LogoutUseCase] = LogoutUseCase(get<AuthRepository>());
    _instances[ForgotPasswordUseCase] = ForgotPasswordUseCase(get<AuthRepository>());

    // Dashboard
    _instances[DashboardRepositoryMock] = DashboardRepositoryMock();

    // Cases (verification tasks) — real backend
    _instances[CasesRepository] = CasesRepositoryHttp(get<ApiClient>());

    // Collections
    _instances[CollectionsRepositoryMock] = CollectionsRepositoryMock();

    // Notifications — real backend
    _instances[NotificationsRepositoryHttp] = NotificationsRepositoryHttp(get<ApiClient>());

    // Visit verification / history
    final visitBox = await Hive.openBox<Map>('visit_history');
    _instances[VisitHistoryRepository] = VisitHistoryRepository(visitBox);

    // Recovery — case listing/report submission on the real backend; quick
    // actions, visit/payment/PDF flows have no backend equivalent and stay
    // local/mock (see RecoveryCaseRepositoryHttp doc comment).
    _instances[RecoveryCaseRepository] = RecoveryCaseRepositoryHttp(get<ApiClient>());
    _instances[VisitRepositoryMock] = VisitRepositoryMock();
    _instances[PaymentRepositoryMock] = PaymentRepositoryMock();
    _instances[CollectionRepositoryMock] = CollectionRepositoryMock(get<RecoveryCaseRepository>());
  }
}
