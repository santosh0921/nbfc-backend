import 'package:dio/dio.dart';
import 'package:pretty_dio_logger/pretty_dio_logger.dart';
import '../config/app_config.dart';
import '../storage/secure_storage_service.dart';

/// Thin Dio wrapper. Every feature repository routes through this so
/// swapping the real Go backend in later is a datasource swap, not a
/// rewrite.
class ApiClient {
  ApiClient(this._secureStorage)
      : dio = Dio(BaseOptions(
          baseUrl: AppConfig.current.apiBaseUrl,
          connectTimeout: const Duration(seconds: 15),
          receiveTimeout: const Duration(seconds: 15),
        )) {
    dio.interceptors.addAll([
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _secureStorage.read('auth_token');
          if (token != null) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          handler.next(options);
        },
        onError: (error, handler) {
          // A session is restored from whatever JWT is sitting in secure
          // storage with no server round-trip to check it's still good
          // (see AuthProvider._restoreSession) — so an employee's 12h-old
          // token, or one invalidated by a backend secret rotation, left
          // the app sitting on the dashboard showing "logged in" while
          // every real request silently 401'd with "Invalid Token" behind
          // the scenes. This is what actually looked like "login isn't
          // working" — the login screen itself was fine, but a stale
          // restored session had no way back to it. Any 401 now forces a
          // clean logout so the router's redirect chain drops the user
          // back on the login screen instead of leaving them stuck on a
          // dead session. The login endpoint's own 401 (wrong password)
          // is excluded — that's a normal in-form error, not a dead
          // session, and is already handled by AuthRemoteDataSource.
          if (error.response?.statusCode == 401 && error.requestOptions.path != '/employee/login') {
            onUnauthorized?.call();
          }
          handler.next(error);
        },
      ),
      if (AppConfig.current.env == AppEnv.development)
        PrettyDioLogger(requestBody: true, responseBody: true, compact: true),
    ]);
  }

  final Dio dio;
  final SecureStorageService _secureStorage;

  /// Fired once when any request comes back 401 (other than a login
  /// attempt itself) — wired in providers.dart to [AuthProvider.logout],
  /// which flips auth status and lets the router's redirect chain send
  /// the employee back to the login screen.
  void Function()? onUnauthorized;
}
