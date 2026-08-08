import 'package:equatable/equatable.dart';

class NomineeInfo extends Equatable {
  const NomineeInfo({required this.name, required this.relation, required this.phone, required this.dob});

  final String name;
  final String relation;
  final String phone;
  final String dob;

  Map<String, dynamic> toMap() => {'name': name, 'relation': relation, 'phone': phone, 'dob': dob};

  factory NomineeInfo.fromMap(Map map) => NomineeInfo(
        name: map['name'] as String? ?? '',
        relation: map['relation'] as String? ?? '',
        phone: map['phone'] as String? ?? '',
        dob: map['dob'] as String? ?? '',
      );

  @override
  List<Object?> get props => [name, relation, phone, dob];
}

class CapturedDocument extends Equatable {
  const CapturedDocument({required this.type, required this.filePath});

  final String type;
  final String filePath;

  Map<String, dynamic> toMap() => {'type': type, 'filePath': filePath};

  factory CapturedDocument.fromMap(Map map) =>
      CapturedDocument(type: map['type'] as String? ?? '', filePath: map['filePath'] as String? ?? '');

  @override
  List<Object?> get props => [type, filePath];
}

class GeoPhoto extends Equatable {
  const GeoPhoto({required this.filePath, required this.latitude, required this.longitude, required this.capturedAt});

  final String filePath;
  final double latitude;
  final double longitude;
  final DateTime capturedAt;

  Map<String, dynamic> toMap() => {
        'filePath': filePath,
        'latitude': latitude,
        'longitude': longitude,
        'capturedAt': capturedAt.toIso8601String(),
      };

  factory GeoPhoto.fromMap(Map map) => GeoPhoto(
        filePath: map['filePath'] as String? ?? '',
        latitude: (map['latitude'] as num?)?.toDouble() ?? 0,
        longitude: (map['longitude'] as num?)?.toDouble() ?? 0,
        capturedAt: DateTime.tryParse(map['capturedAt'] as String? ?? '') ?? DateTime.now(),
      );

  @override
  List<Object?> get props => [filePath, latitude, longitude, capturedAt];
}

/// A completed field-verification visit — the durable record of everything
/// captured during a customer visit (details, nominees, documents, the
/// geotagged site photo and both signatures). Persisted to Hive so it
/// survives app restarts; the Go backend sync point once it exists.
class VisitRecord extends Equatable {
  const VisitRecord({
    required this.id,
    required this.caseId,
    required this.customerName,
    required this.loanType,
    required this.submittedAt,
    required this.occupation,
    required this.monthlyIncome,
    required this.remarks,
    required this.nominees,
    required this.documents,
    required this.geoPhoto,
    required this.customerSignaturePath,
    required this.employeeSignaturePath,
    required this.recommendation,
    this.pdfPath,
  });

  final String id;
  final String caseId;
  final String customerName;
  final String loanType;
  final DateTime submittedAt;
  final String occupation;
  final String monthlyIncome;
  final String remarks;
  final List<NomineeInfo> nominees;
  final List<CapturedDocument> documents;
  final GeoPhoto? geoPhoto;
  final String? customerSignaturePath;
  final String? employeeSignaturePath;
  final String recommendation;
  final String? pdfPath;

  Map<String, dynamic> toMap() => {
        'id': id,
        'caseId': caseId,
        'customerName': customerName,
        'loanType': loanType,
        'submittedAt': submittedAt.toIso8601String(),
        'occupation': occupation,
        'monthlyIncome': monthlyIncome,
        'remarks': remarks,
        'nominees': nominees.map((n) => n.toMap()).toList(),
        'documents': documents.map((d) => d.toMap()).toList(),
        'geoPhoto': geoPhoto?.toMap(),
        'customerSignaturePath': customerSignaturePath,
        'employeeSignaturePath': employeeSignaturePath,
        'recommendation': recommendation,
        'pdfPath': pdfPath,
      };

  factory VisitRecord.fromMap(Map map) => VisitRecord(
        id: map['id'] as String,
        caseId: map['caseId'] as String? ?? '',
        customerName: map['customerName'] as String? ?? '',
        loanType: map['loanType'] as String? ?? '',
        submittedAt: DateTime.tryParse(map['submittedAt'] as String? ?? '') ?? DateTime.now(),
        occupation: map['occupation'] as String? ?? '',
        monthlyIncome: map['monthlyIncome'] as String? ?? '',
        remarks: map['remarks'] as String? ?? '',
        nominees: ((map['nominees'] as List?) ?? []).map((e) => NomineeInfo.fromMap(e as Map)).toList(),
        documents: ((map['documents'] as List?) ?? []).map((e) => CapturedDocument.fromMap(e as Map)).toList(),
        geoPhoto: map['geoPhoto'] != null ? GeoPhoto.fromMap(map['geoPhoto'] as Map) : null,
        customerSignaturePath: map['customerSignaturePath'] as String?,
        employeeSignaturePath: map['employeeSignaturePath'] as String?,
        recommendation: map['recommendation'] as String? ?? '',
        pdfPath: map['pdfPath'] as String?,
      );

  VisitRecord copyWith({String? pdfPath}) => VisitRecord(
        id: id,
        caseId: caseId,
        customerName: customerName,
        loanType: loanType,
        submittedAt: submittedAt,
        occupation: occupation,
        monthlyIncome: monthlyIncome,
        remarks: remarks,
        nominees: nominees,
        documents: documents,
        geoPhoto: geoPhoto,
        customerSignaturePath: customerSignaturePath,
        employeeSignaturePath: employeeSignaturePath,
        recommendation: recommendation,
        pdfPath: pdfPath ?? this.pdfPath,
      );

  @override
  List<Object?> get props => [
        id,
        caseId,
        customerName,
        loanType,
        submittedAt,
        occupation,
        monthlyIncome,
        remarks,
        nominees,
        documents,
        geoPhoto,
        customerSignaturePath,
        employeeSignaturePath,
        recommendation,
        pdfPath,
      ];
}
