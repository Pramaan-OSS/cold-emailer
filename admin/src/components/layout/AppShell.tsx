import {Outlet,Link,useLocation} from "react-router-dom";
import {ConfirmProvider} from "@/components/ConfirmDialog";
import {ReauthDialog} from "@/components/ReauthDialog";
import {useDocumentTitle} from "@/hooks/useDocumentTitle";
import {useMe} from "@/hooks/useMe";
import {Logo} from "@/components/Logo";
import {CommandPalette} from "./CommandPalette";
import {UserMenu} from "./UserMenu";
import {visibleNavGroups,sectionActive} from "./nav";
export function AppShell(){
 useDocumentTitle();const {pathname}=useLocation();const {data:me}=useMe();
 const groups=visibleNavGroups(me?.admin_permissions);
 return <ConfirmProvider><div className="ce-admin"><header className="top"><Link to="/" className="brand"><Logo className="size-7"/>Cold Email</Link><span>Operations</span><UserMenu compact/></header><main className="ce-main" data-thread-none><aside className="left"><div className="strip">Changes are saved to this instance.</div><nav className="msgs admin-nav" aria-label="Operations">{groups.map(group=><div key={group.label}><p>{group.label}</p>{group.items.map(item=><Link className="tab" aria-current={sectionActive(item,pathname)?"page":undefined} key={item.label} to={item.pages[0].to}>{item.label}</Link>)}</div>)}</nav></aside><section className="right"><div className="view"><Outlet/></div></section></main></div><CommandPalette/><ReauthDialog/></ConfirmProvider>;
}
