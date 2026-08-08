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
  /// issued, so the repository can persist both. `module` is only
  /// consulted by the backend when `name` doesn't match any existing
  /// employee (first-time auto-registration) — harmless to always send.
  ///
  /// When [AppConfig.useMockData] is on, login never touches the network —
  /// any non-empty name+password succeeds locally, so signing in doesn't
  /// depend on the backend being reachable (every other call still does).
  /// The employee code is derived deterministically from the name so the
  /// same person gets the same code across sessions on this device.
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
