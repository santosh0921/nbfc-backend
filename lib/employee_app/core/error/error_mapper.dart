import 'package:dio/dio.dart';
import 'failure.dart';

class ErrorMapper {
  ErrorMapper._();

  static Failure map(Object error) {
    if (error is Failure) return error;
    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.receiveTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.connectionError:
          return const NetworkFailure();
        case DioExceptionType.badResponse:
          if (error.response?.statusCode == 401) return const UnauthorizedFailure();
          return const ServerFailure();
        default:
          return const UnknownFailure();
      }
    }
    return const UnknownFailure();
  }
}
