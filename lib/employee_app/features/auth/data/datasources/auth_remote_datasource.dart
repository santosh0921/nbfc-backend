import 'package:dio/dio.dart';
import '../../../../core/config/app_config.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/network/api_client.dart';
import '../../domain/entities/employee_entity.dart';
import '../models/employee_model.dart';

/// Real HTTP datasource for `POST /employee/login` on the Go backend (see
/// internal/employee/login_handler.go). Replaces [AuthMockDataSource] — the
/// repository/usecase layers above are unchanged.
class AuthRemoteDataSource {
  AuthRemoteDataSource(this._apiClient);

  final ApiClient _apiClient;

  /// Returns the logged-in employee together with the JWT the backend
  /// issued, so the repository can persist both. Employee accounts are
  /// admin-provisioned only — `module` is accepted for wire-format
  /// compatibility but the backend no longer acts on it.
  ///
  /// When [AppConfig.useMockData] is on, login never touches the network —
  /// any non-empty name+password succeeds locally, so signing in doesn't
  /// depend on the backend being reachable (every other call still does).
  /// This is strictly a local dev/offline-testing aid: both the
  /// `development` and `production` [AppConfig]s currently ship with
  /// `useMockData: false`, so this path isn't reachable in any built app —
  /// it mirrors the backend's real admin-provisioned accounts, not
  /// self-registration, only in that it never rejects an unrecognized
  /// name (there's no real employee table to check locally).
  Future<({EmployeeModel employee, String token})> login({
    required String name,
    required String password,
    required String module,
  }) async {
    if (AppConfig.current.useMockData) {
      return _mockLogin(name: name, password: password, module: module);
    }
    try {
      final response = await _apiClient.dio.post('/employee/login', data: {
        'name': name.trim(),
        'password': password,
        'module': module,
      });
      final data = response.data as Map<String, dynamic>;
      final employee = EmployeeModel.fromBackendJson(data['employee'] as Map<String, dynamic>);
      final token = data['token'] as String;
      return (employee: employee, token: token);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        final message = (e.response?.data is Map) ? (e.response!.data['message'] as String?) : null;
        throw UnauthorizedFailure(message ?? 'Invalid credentials.');
      }
      if (e.response?.statusCode == 403) {
        final message = (e.response?.data is Map) ? (e.response!.data['message'] as String?) : null;
        throw UnauthorizedFailure(message ?? 'Account deactivated.');
      }
      rethrow;
    }
  }

  /// POST /employee/register — genuine self-service sign-up. Returns
  /// nothing to log in with: the account is created pending admin
  /// approval (Approved=false server-side), so there's no session to
  /// start yet. Throws [ConflictFailure]/[BadRequestFailure]-style errors
  /// via the same DioException mapping the rest of this datasource uses.
  Future<void> register({
    required String name,
    required String password,
    required String role,
    required String branchCity,
  }) async {
    if (AppConfig.current.useMockData) {
      await Future<void>.delayed(const Duration(milliseconds: 500));
      return;
    }
    try {
      await _apiClient.dio.post('/employee/register', data: {
        'name': name.trim(),
        'password': password,
        'role': role,
        'branchCity': branchCity.trim(),
      });
    } on DioException catch (e) {
      final message = (e.response?.data is Map) ? (e.response!.data['message'] as String?) : null;
      throw ValidationFailure(message ?? 'Could not create account.');
    }
  }

  Future<void> forgotPassword(String identifier) async {
    // No backend endpoint exists for this yet; kept as a local no-op so the
    // existing "forgot password" UI flow doesn't need to change.
    await Future.delayed(const Duration(milliseconds: 400));
  }

  Future<({EmployeeModel employee, String token})> _mockLogin({
    required String name,
    required String password,
    required String module,
  }) async {
    await Future<void>.delayed(const Duration(milliseconds: 500));
    final trimmedName = name.trim();
    if (trimmedName.isEmpty || password.trim().isEmpty) {
      throw const UnauthorizedFailure('Enter your name and password.');
    }

    // Deterministic per-name code (not cryptographic — just needs to be
    // stable across logins on this device) so the same person always gets
    // the same employee code back.
    var hash = 0;
    for (final unit in trimmedName.toLowerCase().codeUnits) {
      hash = (hash * 31 + unit) & 0x7fffffff;
    }
    final code = 'EMP-${hash.toRadixString(16).padLeft(6, '0').substring(0, 6).toUpperCase()}';

    final role = module == 'recovery' ? EmployeeRole.collectionAgent : EmployeeRole.fieldVerificationOfficer;
    final employee = EmployeeModel(
      id: code,
      employeeCode: code,
      name: trimmedName,
      email: '${code.toLowerCase()}@onefin.com',
      role: role,
      designation: role.designationLabel,
      branch: 'Mumbai',
    );
    return (employee: employee, token: 'mock-token-$code');
  }
}
