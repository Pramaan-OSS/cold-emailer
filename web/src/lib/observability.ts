export type Identity = { userId: string; email?: string|null; name?: string|null; organizationId?: string|null; organizationName?: string|null; plan?: string|null } | null;
export type StepProperties = Record<string,string|number|boolean>;
export function initErrorReporting(): void {}
export function captureException(_error:unknown): void {}
export function setErrorIdentity(_next:Identity): void {}
export function noteStep(_message:string,_properties?:StepProperties): void {}
