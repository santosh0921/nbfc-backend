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
        case DioExceptionType.unknown:
          // Dio's catch-all for a request that never got as far as an HTTP
          // response at all — in practice this is almost always a raw
          // SocketException/HandshakeException from the platform (DNS
          // failure, connection refused, TLS handshake broken on this
          // specific device/network), which is exactly what NetworkFailure
          // is for. It used to fall through to the `default` branch below
          // and show a completely uninformative "An unexpected error
          // occurred." with no indication it was a connectivity problem at
          // all — this is what surfaced on-device as an opaque login
          // failure with no way to tell it apart from a real server bug.
          return const NetworkFailure('Could not reach the server. Check your connection and try again.');
        default:
          return const UnknownFailure();
      }
    }
    return const UnknownFailure();
  }
}
