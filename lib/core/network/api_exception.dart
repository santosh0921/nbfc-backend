/// Thrown by [ApiClient] when the backend returns a non-2xx response or
/// the request itself fails (no connection, timeout, bad JSON). Carries
/// the backend's own `message` field when available so callers can show
/// it directly instead of a generic "something went wrong".
class ApiException implements Exception {
  ApiException(this.message, {this.statusCode});

  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}
