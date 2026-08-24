import { Navigate, Route, Routes } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { AdminAccountsPage } from "../pages/AdminAccountsPage";
import { ChangePasswordPage } from "../pages/ChangePasswordPage";
import { HomePage } from "../pages/HomePage";
import { LoginPage } from "../pages/LoginPage";
import { SystemPage } from "../pages/SystemPage";
import { CharacterPage } from "../pages/CharacterPage";
import { CharactersPage } from "../pages/CharactersPage";
import { RollsPage } from "../pages/RollsPage";
import { CampaignPage } from "../pages/CampaignPage";
import { CampaignsPage } from "../pages/CampaignsPage";
import { AppLayout } from "./AppLayout";

export function App() {
  const { account, loading } = useAuth();
  if (loading) return <div className="center-message">正在加载……</div>;
  if (!account) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }
  if (account.mustChangePassword) {
    return (
      <Routes>
        <Route
          path="/change-password"
          element={<ChangePasswordPage required />}
        />
        <Route path="*" element={<Navigate to="/change-password" replace />} />
      </Routes>
    );
  }
  if (account.role === "admin") {
    return (
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<Navigate to="/admin/accounts" replace />} />
          <Route path="admin/accounts" element={<AdminAccountsPage />} />
          <Route path="admin/system" element={<SystemPage />} />
          <Route path="change-password" element={<ChangePasswordPage />} />
          <Route path="*" element={<Navigate to="/admin/accounts" replace />} />
        </Route>
      </Routes>
    );
  }
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<HomePage />} />
        <Route path="change-password" element={<ChangePasswordPage />} />
        <Route path="characters" element={<CharactersPage />} />
        <Route path="characters/:id" element={<CharacterPage />} />
        <Route path="campaigns" element={<CampaignsPage />} />
        <Route path="campaigns/:id" element={<CampaignPage />} />
        <Route path="rolls" element={<RollsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}
