import 'api_client.dart';
import '../../models/complaint.dart';

/// Wraps the backend's protected `/auth/complaints` endpoints
/// (`internal/complaints/handler.go`) — the real backend the "Raise a
/// Complaint" form and "My Complaints" tab should have been calling all
/// along, instead of keeping entries in an in-memory list that vanished
/// when the app restarted and never reached the admin panel.
class ComplaintApiService {
  ComplaintApiService._();

  static Future<Complaint> create(String token, String category, String description) async {
    final res = await ApiClient.post(
      '/auth/complaints',
      {'category': category, 'description': description},
      token: token,
    );
    return Complaint.fromJson(res);
  }

  static Future<List<Complaint>> listMine(String token) async {
    final res = await ApiClient.getList('/auth/complaints', token: token);
    return res.map((e) => Complaint.fromJson(e as Map<String, dynamic>)).toList();
  }
}
