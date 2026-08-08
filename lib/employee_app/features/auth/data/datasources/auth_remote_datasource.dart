import 'package:dio/dio.dart';
import '../../../../core/error/failure.dart';
import '../../../../core/network/api_client.dart';
import '../models/employee_model.dart';

/// Real HTTP datasource for `POST /employee/login` on the Go backend (see
/// internal/employee/login_handler.go). Replaces [AuthMockDataSource] — the
/// repository/usecase layers above are unchanged.
class AuthRemoteDataSource {
  AuthRemoteDataSource(this._apiClient);

  final ApiClient _apiClient;

  /// Returns the logged-in employee together with the JWT the backend
  /// issued, so the repository can persist both.
  Future<({EmployeeModel employee, String token})> login({
    required String identifier,
    required String password,
  }) async {
    try {
      final response = await _apiClient.dio.post('/employee/login', data: {
        'code': identifier.trim(),
        'password': password,
      });
      final data = response.data as Map<String, dynamic>;
      final employee = EmployeeModel.fromBackendJson(data['employee'] as Map<String, dynamic>);
      final token = data['token'] as String;
      return (employee: employee, token: token);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        throw const UnauthorizedFailure('Invalid credentials.');
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
}
