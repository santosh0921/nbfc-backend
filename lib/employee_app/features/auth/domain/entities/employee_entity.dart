import 'package:equatable/equatable.dart';

enum EmployeeRole { fieldVerificationOfficer, collectionAgent, supervisor }

class EmployeeEntity extends Equatable {
  const EmployeeEntity({
    required this.id,
    required this.employeeCode,
    required this.name,
    required this.email,
    required this.role,
    required this.designation,
    required this.branch,
    this.avatarUrl,
  });

  final String id;
  final String employeeCode;
  final String name;
  final String email;
  final EmployeeRole role;
  final String designation;
  final String branch;
  final String? avatarUrl;

  @override
  List<Object?> get props => [id, employeeCode, name, email, role, designation, branch, avatarUrl];
}

/// Maps between the backend's `Employee.Role` string vocabulary
/// (verification | recovery | supervisor — see internal/models/employee.go)
/// and the app's [EmployeeRole] enum.
extension EmployeeRoleBackend on EmployeeRole {
  static EmployeeRole fromBackend(String role) => switch (role) {
        'recovery' => EmployeeRole.collectionAgent,
        'supervisor' => EmployeeRole.supervisor,
        _ => EmployeeRole.fieldVerificationOfficer,
      };

  String get backendValue => switch (this) {
        EmployeeRole.fieldVerificationOfficer => 'verification',
        EmployeeRole.collectionAgent => 'recovery',
        EmployeeRole.supervisor => 'supervisor',
      };

  String get designationLabel => switch (this) {
        EmployeeRole.fieldVerificationOfficer => 'Field Verification Officer',
        EmployeeRole.collectionAgent => 'Collection Agent',
        EmployeeRole.supervisor => 'Supervisor',
      };
}
