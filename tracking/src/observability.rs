use tracing::error;
pub struct Settings<'a> { pub env: &'a str, pub release: &'a str }
pub struct Guard;
pub fn init(settings: Settings<'_>) -> Guard { let _ = (settings.env,settings.release); Guard }
pub fn flush() {}
pub fn report_issue(context: &str, details: &str) { error!("[issue-local][tracking] {context}: {details}"); }
pub fn report_error(context: &str, err: &(dyn std::error::Error + Send + Sync)) {report_issue(context,&err.to_string());}
