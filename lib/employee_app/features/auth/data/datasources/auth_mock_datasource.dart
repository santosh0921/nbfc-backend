import '../../../../core/error/failure.dart';
import '../../domain/entities/employee_entity.dart';
import '../models/employee_model.dart';

/// Mock datasource — simulates a Go backend auth endpoint with a fixed
/// employee directory and artificial latency. Swap for `AuthRemoteDataSource`
/// (Dio-backed) once the real API is available; the repository/usecase
/// layers above do not need to change.
class AuthMockDataSource {
  static final _directory = <String, ({String password, EmployeeModel employee})>{
    'EMP1001': (
      password: 'onefin123',
      employee: const EmployeeModel(
        id: 'emp-1001',
        employeeCode: 'EMP1001',
        name: 'Rohan Mehta',
        email: 'rohan.mehta@onefin.com',
        role: EmployeeRole.fieldVerificationOfficer,
        designation: 'Field Verification Officer',
        branch: 'Pune – Baner Branch',
      ),
    ),
    'EMP1002': (
      password: 'onefin123',
      employee: const EmployeeModel(
        id: 'emp-1002',
        employeeCode: 'EMP1002',
        name: 'Ananya Sharma',
        email: 'ananya.sharma@onefin.com',
        role: EmployeeRole.collectionAgent,
        designation: 'Collection Agent',
        branch: 'Mumbai – Andheri Branch',
      ),
    ),
    'EMP1003': (
      password: 'onefin123',
      employee: const EmployeeModel(
        id: 'emp-1003',
        employeeCode: 'EMP1003',
        name: 'Vikram Rao',
        email: 'vikram.rao@onefin.com',
        role: EmployeeRole.supervisor,
        designation: 'Verification Supervisor',
        branch: 'Bengaluru – Koramangala Branch',
      ),
    ),
  };

  Future<EmployeeModel> login({required String identifier, required String password}) async {
    await Future.delayed(const Duration(milliseconds: 700));
    final key = identifier.trim().toUpperCase();
    final match = _directory[key] ??
        _directory.values.cast<({String password, EmployeeModel employee})?>().firstWhere(
              (e) => e!.employee.email.toLowerCase() == identifier.trim().toLowerCase(),
              orElse: () => null,
            );
    if (match == null || match.password != password) {
      throw const UnauthorizedFailure('Invalid employee ID / email or password.');
    }
    return match.employee;
  }

  /// Re-authenticates whichever employee last logged in with a password on
  /// this device — [lastEmployeeCode] is persisted by the repository at the
  /// end of a successful [login]. There is no "current biometric user"
  /// concept in a mock directory, so without a prior password login there
  /// is nothing to resolve to.
  Future<EmployeeModel> biometricLogin(String? lastEmployeeCode) async {
    await Future.delayed(const Duration(milliseconds: 500));
    if (lastEmployeeCode == null) {
      throw const UnauthorizedFailure('No previous session found. Please log in with your employee ID.');
    }
    final match = _directory[lastEmployeeCode.trim().toUpperCase()];
    if (match == null) {
      throw const UnauthorizedFailure('No previous session found. Please log in with your employee ID.');
    }
    return match.employee;
  }

  Future<void> forgotPassword(String identifier) async {
    await Future.delayed(const Duration(milliseconds: 600));
  }
}
