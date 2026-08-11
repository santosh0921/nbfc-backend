import 'api_client.dart';
import '../../models/uploaded_document.dart';

/// Wraps the backend's protected `/auth/documents` endpoints
/// (`internal/documents/handler.go`) — uploading a document as the
/// logged-in customer and reading back the customer's own documents.
class DocumentApiService {
  DocumentApiService._();

  static Future<UploadedDocument> upload(
    String token, {
    required String docType,
    required String fileName,
    required String mimeType,
    required String dataBase64,
    int? loanId,
  }) async {
    final res = await ApiClient.post('/auth/documents', {
      'docType': docType,
      'fileName': fileName,
      'mimeType': mimeType,
      'dataBase64': dataBase64,
      if (loanId != null) 'loanId': loanId,
    }, token: token);
    return UploadedDocument.fromJson(res);
  }

  /// [loanId], when given, scopes the result to documents tied to that
  /// specific loan (e.g. verification photos an employee captured for
  /// it) — useful for a customer with more than one loan who shouldn't
  /// see every document across every loan in one undifferentiated list.
  static Future<List<UploadedDocument>> mine(String token, {int? loanId}) async {
    final path = loanId == null ? '/auth/documents' : '/auth/documents?loanId=$loanId';
    final res = await ApiClient.getList(path, token: token);
    return res.map((e) => UploadedDocument.fromJson(e as Map<String, dynamic>)).toList();
  }
}
