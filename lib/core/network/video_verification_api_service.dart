import 'api_client.dart';
import '../../models/video_verification_request.dart';

/// Wraps the backend's customer-auth video-KYC-verification-request
/// endpoints (`internal/kyc` — new endpoints added alongside the KYC
/// portal's "request video verification" button):
///   GET  /kyc/verification-requests
///   POST /kyc/verification-requests/:id/respond
class VideoVerificationApiService {
  VideoVerificationApiService._();

  static Future<List<VideoVerificationRequest>> list(String token) async {
    final res = await ApiClient.getList('/kyc/verification-requests', token: token);
    return res.map((e) => VideoVerificationRequest.fromJson(e as Map<String, dynamic>)).toList();
  }

  static Future<VideoVerificationRequest> respond(
    String token,
    int id, {
    required bool ready,
    required String preferredTime,
  }) async {
    final res = await ApiClient.post(
      '/kyc/verification-requests/$id/respond',
      {'ready': ready, 'preferredTime': preferredTime},
      token: token,
    );
    return VideoVerificationRequest.fromJson(res);
  }
}
