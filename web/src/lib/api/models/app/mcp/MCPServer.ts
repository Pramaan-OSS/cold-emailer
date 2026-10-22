export interface MCPTool { assistant_error?:string; tool_ref:string; schema_digest:string; description:string; input_schema?:unknown; approval_required:boolean }
export interface MCPServer { id:string; org_id:string; name:string; auth_type:'none'|'bearer'|'oauth'; enabled:boolean; read_only:boolean; state:'pending-auth'|'ready'|'failed'; connector_ref:string; connect_url?:string; discovered_tools:MCPTool[]; last_error?:string; created_at:Date; updated_at:Date }
export interface CreateMCPServer { name:string; url:string; auth_type:'none'|'bearer'|'oauth'; token?:string }
export interface UpdateMCPServer { name?:string; enabled?:boolean; read_only?:boolean; allow_without_approval?:string[]; approved?:boolean }
export interface RemoteToolResult { status:'succeeded'|'failed'|'unknown'; output:unknown; truncated?:boolean; errorCode?:string }
