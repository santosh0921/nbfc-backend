import 'api_client.dart';

/// Wraps the backend's protected `/auth/*` KYC/profile endpoints — every
/// call here requires the bearer token from [AuthApiService.login].
class KycApiService {
  KycApiService._();

  static Future<void> createProfile(
    String token, {
    required String firstName,
    required String lastName,
    required String email,
    required String dateOfBirth,
    required String gender,
    required String maritalStatus,
    required String occupation,
  }) async {
    await ApiClient.post('/auth/profile', {
      'first_name': firstName,
      'last_name': lastName,
      'email': email,
      'date_of_birth': dateOfBirth,
      'gender': gender,
      'marital_status': maritalStatus,
      'occupation': occupation,
    }, token: token);
  }

  static Future<void> createAddress(
    String token, {
    required String currentAddress,
    required String currentState,
    required String currentDistrict,
    required String currentCity,
    required String currentPincode,
    bool sameAsCurrent = true,
  }) async {
    await ApiClient.post('/auth/address', {
      'current_address': currentAddress,
      'current_state': currentState,
      'current_district': currentDistrict,
      'current_city': currentCity,
      'current_pincode': currentPincode,
      'same_as_current': sameAsCurrent,
    }, token: token);
  }

  static Future<void> createPan(String token, {required String panNumber, required String nameOnPan}) async {
    await ApiClient.post('/auth/pan', {
      'pan_number': panNumber,
      'name_on_pan': nameOnPan,
    }, token: token);
  }

  static Future<void> createAadhaar(
    String token, {
    required String aadhaarNumber,
    required String nameOnAadhaar,
    required bool consent,
  }) async {
    await ApiClient.post('/auth/aadhaar', {
      'aadhaar_number': aadhaarNumber,
      'name_on_aadhaar': nameOnAadhaar,
      'consent': consent,
    }, token: token);
  }

  static Future<void> createSelfie(String token, {required String imageBase64}) async {
    await ApiClient.post('/auth/selfie', {'image': imageBase64}, token: token);
  }

  static Future<void> createEmployment(
    String token, {
    required String employmentType,
    required String companyName,
    required String occupation,
    required double monthlyIncome,
    String workEmail = '',
    int experienceYears = 0,
    int salaryDay = 1,
  }) async {
    await ApiClient.post('/auth/employment', {
      'employment_type': employmentType,
      'company_name': companyName,
      'occupation': occupation,
      'monthly_income': monthlyIncome,
      'work_email': workEmail,
      'experience_years': experienceYears,
      'salary_day': salaryDay,
    }, token: token);
  }

  static Future<Map<String, dynamic>> getKycStatus(String token) async {
    return ApiClient.get('/auth/kyc/status', token: token);
  }
}
