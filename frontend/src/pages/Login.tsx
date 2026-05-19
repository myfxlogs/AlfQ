// Login page — ALFQ authentication.
import { useState } from "react";
import { authClient, setToken } from "../api/client";

export default function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    setError("");
    setLoading(true);
    try {
      const res = await authClient.login({ email, password });
      if (res.accessToken) {
        setToken(res.accessToken);
        window.location.href = "/";
      } else {
        setError("登录失败：未返回 token");
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ display:"flex", justifyContent:"center", alignItems:"center", minHeight:"100vh", background:"#f0f2f5" }}>
      <div style={{ background:"#fff", padding:"3rem", borderRadius:12, boxShadow:"0 4px 12px rgba(0,0,0,0.1)", width:360 }}>
        <h1 style={{ textAlign:"center", marginBottom:"2rem" }}>ALFQ</h1>
        {error && <div style={{ color:"red", marginBottom:12, fontSize:14 }}>{error}</div>}
        <input placeholder="邮箱" type="email" value={email} onChange={e=>setEmail(e.target.value)}
          style={{ width:"100%", padding:12, marginBottom:12, border:"1px solid #ddd", borderRadius:6 }} />
        <input placeholder="密码" type="text" value={password} onChange={e=>setPassword(e.target.value)}
          style={{ width:"100%", padding:12, marginBottom:20, border:"1px solid #ddd", borderRadius:6 }} />
        <button onClick={handleLogin} disabled={loading}
          style={{ width:"100%", padding:12, background:"#1677ff", color:"#fff", border:"none", borderRadius:6, fontSize:16, cursor:"pointer", opacity: loading ? 0.7 : 1 }}>
          {loading ? "登录中..." : "登录"}
        </button>
      </div>
    </div>
  );
}
