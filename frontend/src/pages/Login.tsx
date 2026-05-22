// Login page — ALFQ authentication.
import { useState } from "react";
import { authClient, saveAuth } from "../api/client";

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
        saveAuth(res.accessToken, res.refreshToken, Number(res.expiresIn));
        localStorage.setItem("alfq_email", email);
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
    <div style={{ display:"flex", justifyContent:"center", alignItems:"center", minHeight:"100vh", background:"var(--color-bg-secondary)" }}>
      <div className="glass-card" style={{ padding:"3rem", width:360 }}>
        <h1 className="text-gradient" style={{ textAlign:"center", marginBottom:"2rem", fontSize:28 }}>ALFQ</h1>
        {error && <div style={{ color:"var(--color-danger)", marginBottom:12, fontSize:14 }}>{error}</div>}
        <input className="input" placeholder="邮箱" type="email" value={email} onChange={e=>setEmail(e.target.value)}
          style={{ width:"100%", marginBottom:12 }} />
        <input className="input" placeholder="密码" type="password" value={password} onChange={e=>setPassword(e.target.value)}
          style={{ width:"100%", marginBottom:20 }} />
        <button className="btn-primary" onClick={handleLogin} disabled={loading}
          style={{ width:"100%", padding:12, fontSize:16, opacity: loading ? 0.7 : 1 }}>
          {loading ? "登录中..." : "登录"}
        </button>
      </div>
    </div>
  );
}
