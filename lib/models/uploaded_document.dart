/// A customer-uploaded document as returned by the backend
/// (`internal/models/document.go` `Document`), serialized by
/// `POST /auth/documents` and `GET /auth/documents`
/// (`internal/documents/handler.go`). Field names mirror the backend's
/// JSON tags exactly (camelCase) — see that struct before changing
/// anything here.
///
/// Note `id` and `customerId` are UUID strings (Postgres `uuid` columns),
/// not ints — unlike [LoanApplication.id].
class UploadedDocument {
  final String id;
  final String customerId;
  final int? loanId;
  final String docType;
  final String fileName;
  final String mimeType;
  final String dataBase64;
  final String uploadedByType;
  final String uploadedByRef;
  final DateTime createdAt;

  const UploadedDocument({
    required this.id,
    required this.customerId,
    this.loanId,
    required this.docType,
    required this.fileName,
    required this.mimeType,
    required this.dataBase64,
    required this.uploadedByType,
    required this.uploadedByRef,
    required this.createdAt,
  });

  bool get isImage => mimeType.startsWith('image/');
  bool get isPdf => mimeType == 'application/pdf';

  factory UploadedDocument.fromJson(Map<String, dynamic> json) {
    return UploadedDocument(
      id: json['id'] as String? ?? '',
      customerId: json['customerId'] as String? ?? '',
      loanId: json['loanId'] as int?,
      docType: json['docType'] as String? ?? '',
      fileName: json['fileName'] as String? ?? '',
      mimeType: json['mimeType'] as String? ?? '',
      dataBase64: json['dataBase64'] as String? ?? '',
      uploadedByType: json['uploadedByType'] as String? ?? '',
      uploadedByRef: json['uploadedByRef'] as String? ?? '',
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
    );
  }
}
