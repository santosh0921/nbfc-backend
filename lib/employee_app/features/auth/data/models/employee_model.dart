import '../../domain/entities/employee_entity.dart';

class EmployeeModel extends EmployeeEntity {
  const EmployeeModel({
    required super.id,
    required super.employeeCode,
    required super.name,
    required super.email,
    required super.role,
    required super.designation,
    required super.branch,
    super.avatarUrl,
  });

  /// Parses the `employee` object embedded in the backend's
  /// `POST /employee/login` response (see internal/models/employee.go):
  /// `{id, code, name, role, branchCity, agencyId, active, createdAt}`.
  /// The backend has no `email` field for employees, so a synthetic one is
  /// derived from the employee code for display purposes only.
  factory EmployeeModel.fromBackendJson(Map<String, dynamic> json) {
    final role = EmployeeRoleBackend.fromBackend(json['role'] as String? ?? 'verification');
    final code = json['code'] as String? ?? '';
    return EmployeeModel(
      id: '${json['id']}',
      employeeCode: code,
      name: json['name'] as String? ?? code,
      email: '${code.toLowerCase()}@onefin.com',
      role: role,
      designation: role.designationLabel,
      branch: json['branchCity'] as String? ?? '',
    );
  }

  /// Round-trips the locally persisted session (secure storage) — kept in
  /// the app's own JSON shape, independent of the backend's wire format.
  factory EmployeeModel.fromJson(Map<String, dynamic> json) => EmployeeModel(
        id: json['id'] as String,
        employeeCode: json['employeeCode'] as String,
        name: json['name'] as String,
        email: json['email'] as String,
        role: EmployeeRole.values.byName(json['role'] as String),
        designation: json['designation'] as String,
        branch: json['branch'] as String,
        avatarUrl: json['avatarUrl'] as String?,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'employeeCode': employeeCode,
        'name': name,
        'email': email,
        'role': role.name,
        'designation': designation,
        'branch': branch,
        'avatarUrl': avatarUrl,
      };
}
