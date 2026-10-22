pub struct ParameterStore;
impl ParameterStore {
 pub async fn get(&self,_name:&str)->Result<String,Box<dyn std::error::Error+Send+Sync>>{Err("Use local environment configuration.".into())}
 pub async fn get_optional(&self,name:&str)->Option<String>{self.get(name).await.ok()}
}
