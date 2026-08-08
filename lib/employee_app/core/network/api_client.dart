import 'package:dio/dio.dart';
import 'package:pretty_dio_logger/pretty_dio_logger.dart';
import '../config/app_config.dart';
import '../storage/secure_storage_service.dart';

/// Thin Dio wrapper. Not called anywhere yet (app runs on mock repositories)
/// but every future feature repository should route through this so wiring
/// the real Go backend later is a datasource swap, not a rewrite.
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
      ),
      if (AppConfig.current.env == AppEnv.development)
        PrettyDioLogger(requestBody: true, responseBody: true, compact: true),
    ]);
  }

  final Dio dio;
  final SecureStorageService _secureStorage;
}
