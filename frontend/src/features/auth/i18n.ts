// Sign-in / registration strings.
// English is the source of truth; LocaleKey is derived from the merged
// dictionaries in shared/i18n/dictionaries.ts.
export const authEn = {
  auth_email_exists: "An account with that email already exists. Sign in instead.",
  auth_invalid_credentials: "Email or password is incorrect.",
  auth_invalid_input: "Enter a valid email and a password of at least 8 characters.",
  auth_request_failed: "Could not reach the authentication service. Please try again.",
  create_account: "Create account",
  login_account_label: "Email",
  login_card_desc: "Sign in with the email address and password you registered with.",
  login_card_title: "Sign in",
  login_footer_info: "Passwords are securely hashed. Your session lasts 30 days.",
  login_prompt: "Already have an account?",
  password_placeholder: "Password",
  register_card_desc: "Register with your email and a password to get started.",
  register_card_title: "Create your account",
  register_password_hint: "At least 8 characters",
  register_prompt: "New to Codritium?",
  sign_in: "Sign in",
} satisfies Record<string, string>;
