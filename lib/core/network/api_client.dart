import 'dart:convert';
import 'package:http/http.dart' as http;
import 'api_exception.dart';

/// Thin JSON REST client for the Go/Gin backend in `cmd/api`. Reached at
/// `http://localhost:8080` — on a physical device that only works behind
/// `adb reverse tcp:8080 tcp:8080` (which forwards the device's own
/// localhost:8080 to the dev machine's), the standard trick for talking
/// to a local backend from a USB-tethered device without needing to know
/// its LAN IP.
class ApiClient {
  ApiClient._();

  static const String baseUrl = 'http://localhost:8080';
  static const Duration _timeout = Duration(seconds: 15);

  static Map<String, String> _headers(String? token) => {
        'Content-Type': 'application/json',
        if (token != null) 'Authorization': 'Bearer $token',
      };

  static Future<Map<String, dynamic>> get(String path, {String? token}) async {
    return _send(() => http.get(Uri.parse('$baseUrl$path'), headers: _headers(token)));
  }

  /// Same as [get], but for endpoints that return a bare JSON array at the
  /// top level (e.g. `/auth/loans/mine`, `/notifications`) rather than an
  /// object.
  static Future<List<dynamic>> getList(String path, {String? token}) async {
    return _sendList(() => http.get(Uri.parse('$baseUrl$path'), headers: _headers(token)));
  }

  static Future<Map<String, dynamic>> post(String path, Map<String, dynamic> body, {String? token}) async {
    return _send(() => http.post(Uri.parse('$baseUrl$path'), headers: _headers(token), body: jsonEncode(body)));
  }

  static Future<Map<String, dynamic>> put(String path, Map<String, dynamic> body, {String? token}) async {
    return _send(() => http.put(Uri.parse('$baseUrl$path'), headers: _headers(token), body: jsonEncode(body)));
  }

  static Future<Map<String, dynamic>> _send(Future<http.Response> Function() request) async {
    final response = await _execute(request);

    Map<String, dynamic> decoded;
    try {
      decoded = response.body.isEmpty ? {} : jsonDecode(response.body) as Map<String, dynamic>;
    } catch (_) {
      throw ApiException('Unexpected response from server.', statusCode: response.statusCode);
    }

    _throwIfError(response, decoded);
    return decoded;
  }

  static Future<List<dynamic>> _sendList(Future<http.Response> Function() request) async {
    final response = await _execute(request);

    if (response.statusCode < 200 || response.statusCode >= 300) {
      Map<String, dynamic> decoded;
      try {
        decoded = response.body.isEmpty ? {} : jsonDecode(response.body) as Map<String, dynamic>;
      } catch (_) {
        decoded = {};
      }
      _throwIfError(response, decoded);
    }

    try {
      return response.body.isEmpty ? [] : jsonDecode(response.body) as List<dynamic>;
    } catch (_) {
      throw ApiException('Unexpected response from server.', statusCode: response.statusCode);
    }
  }

  static Future<http.Response> _execute(Future<http.Response> Function() request) async {
    try {
      return await request().timeout(_timeout);
    } catch (_) {
      throw ApiException('Could not reach the server. Please check your connection and try again.');
    }
  }

  static void _throwIfError(http.Response response, Map<String, dynamic> decoded) {
    if (response.statusCode < 200 || response.statusCode >= 300) {
      final message = decoded['message'] as String? ?? 'Request failed (${response.statusCode}).';
      throw ApiException(message, statusCode: response.statusCode);
    }
  }
}
