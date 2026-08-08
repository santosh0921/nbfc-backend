enum AppEnv { development, staging, production }

class AppConfig {
  const AppConfig._({required this.env, required this.apiBaseUrl, required this.useMockData});

  final AppEnv env;
  final String apiBaseUrl;
  final bool useMockData;

  static late AppConfig current;

  static void init(AppConfig config) {
    current = config;
  }

  // Points at the deployed Render backend — same one the customer app's
  // ApiClient (lib/core/network/api_client.dart) and the admin panel use —
  // so login and every other call go to the real, always-on backend
  // instead of a local `go run` process that needs to be kept running and
  // (on a physical device) `adb reverse tcp:8080 tcp:8080` kept alive.
  static const development = AppConfig._(
    env: AppEnv.development,
    apiBaseUrl: 'https://nbfc-backend-gr1t.onrender.com',
    useMockData: false,
  );

  static const production = AppConfig._(
    env: AppEnv.production,
    apiBaseUrl: 'https://api.onefin.com/v1',
    useMockData: false,
  );
}
