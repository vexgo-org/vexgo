import { useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import type { User as UserType } from "@/types";

import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { PageHeader } from "@/components/PageHeader";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Spinner } from "@/components/ui/spinner";
import {
  User,
  Mail,
  Key,
  Check,
  Calendar,
  Eye,
  EyeOff,
  Camera,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import ImageCropper from "@/components/image/ImageCropper";
import { isAxiosError } from "axios";

/**
 * The password inputs are identical except for their target state, so they
 * share one component instead of three copies of the same markup.
 */
function PasswordField({
  id,
  label,
  value,
  onChange,
  visible,
  onToggleVisibility,
  autoComplete,
  minLength,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  visible: boolean;
  onToggleVisibility: () => void;
  autoComplete?: string;
  minLength?: number;
}) {
  const { t } = useTranslation();

  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Key className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
        <Input
          id={id}
          type={visible ? "text" : "password"}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="pr-9 pl-9"
          autoComplete={autoComplete}
          minLength={minLength}
          required
        />
        <button
          type="button"
          onClick={onToggleVisibility}
          aria-label={
            visible ? t("loginPage.hidePassword") : t("loginPage.showPassword")
          }
          className="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2 transition-colors"
        >
          {visible ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
    </div>
  );
}

export function ProfilePage() {
  const { user, updateUser } = useAuth();
  const { t } = useTranslation();
  const [username, setUsername] = useState(user?.username || "");
  const [birthday, setBirthday] = useState(user?.birthday || "");
  const [bio, setBio] = useState(user?.bio || "");
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [emailLoading, setEmailLoading] = useState(false);
  const [success, setSuccess] = useState("");
  const [error, setError] = useState("");
  const [showEmailDialog, setShowEmailDialog] = useState(false);
  const [showOldPassword, setShowOldPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [avatarLoading, setAvatarLoading] = useState(false);
  const [showCropper, setShowCropper] = useState(false);
  const [selectedAvatarFile, setSelectedAvatarFile] = useState<File | null>(
    null,
  );

  const handleUpdateProfile = async (e: React.SubmitEvent) => {
    e.preventDefault();
    setError("");
    setSuccess("");
    setLoading(true);

    try {
      const response = await unwrap(
        getVexGoAPI().putAuthProfile({
          username,
          birthday,
          bio,
        }),
      );
      updateUser(response.user as UserType);
      setSuccess(t("profilePage.updateSuccess"));
    } catch (err: unknown) {
      const errorMessage =
        err instanceof Error ? err.message : t("profilePage.updateFailed");
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const handleChangePassword = async (e: React.SubmitEvent) => {
    e.preventDefault();
    setError("");
    setSuccess("");

    if (newPassword !== confirmPassword) {
      setError(t("profilePage.passwordMismatch"));
      return;
    }

    if (newPassword.length < 6) {
      setError(t("profilePage.passwordTooShort"));
      return;
    }

    setPasswordLoading(true);

    try {
      await unwrap(getVexGoAPI().putAuthPassword({ oldPassword, newPassword }));
      setSuccess(t("profilePage.passwordChangeSuccess"));
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err: unknown) {
      let errorMessage = t("profilePage.passwordChangeFailed");
      if (isAxiosError(err)) {
        switch (err.response?.status) {
          case 400:
            errorMessage = t("profilePage.invalidPayload");
            break;
          case 403:
            errorMessage = t("profilePage.currentPasswordIncorrect");
            break;
        }
      }

      setError(errorMessage);
    } finally {
      setPasswordLoading(false);
    }
  };

  const handleUpdateEmail = async (e: React.MouseEvent) => {
    e.preventDefault();
    setError("");
    setSuccess("");
    setShowEmailDialog(false);

    if (!newEmail) {
      setError(t("profilePage.enterNewEmail"));
      return;
    }

    if (newEmail === user?.email) {
      setError(t("profilePage.emailSameAsCurrent"));
      return;
    }

    setEmailLoading(true);

    try {
      const response = await unwrap(
        getVexGoAPI().putAuthEmail({ email: newEmail }),
      );
      setSuccess(response.message ?? "");
      setNewEmail("");

      let newUserEmail = newEmail;
      if ("user" in response && response.user) {
        // If the update succeeded directly (SMTP disabled), update the local user
        newUserEmail = response.user.email ?? newEmail;
      }

      // If pending: true is returned, email verification is required; wait for the user to click the link
      // No need to update the local user; it will be updated after verification
      if (!response.pending && user) {
        updateUser({ ...user, email: newUserEmail });
      }
    } catch (err: unknown) {
      const errorMessage =
        err instanceof Error ? err.message : t("profilePage.emailChangeFailed");
      setError(errorMessage);
    } finally {
      setEmailLoading(false);
    }
  };

  const openEmailChangeDialog = () => {
    setError("");
    setSuccess("");
    setShowEmailDialog(true);
  };

  const handleAvatarClick = () => {
    document.getElementById("avatar-upload")?.click();
  };

  const handleAvatarChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setSelectedAvatarFile(file);
    setShowCropper(true);
  };

  const handleAvatarCrop = async (croppedFile: File) => {
    setAvatarLoading(true);
    setShowCropper(false);

    try {
      // Use the existing upload API
      const uploadResponse = await unwrap(
        getVexGoAPI().postUpload({ file: croppedFile }),
      );
      if (uploadResponse.file && uploadResponse.file.url) {
        // Update the user avatar
        const updateResponse = await unwrap(
          getVexGoAPI().putAuthProfile({
            avatar: uploadResponse.file.url,
          }),
        );
        updateUser(updateResponse.user as UserType);
        setSuccess(t("profilePage.updateAvatar"));
      }
    } catch (err: unknown) {
      const errorMessage =
        err instanceof Error
          ? err.message
          : t("profilePage.avatarUpdateFailed");
      setError(errorMessage);
    } finally {
      setAvatarLoading(false);
      setSelectedAvatarFile(null);
    }
  };

  const getRoleLabel = (role?: string) => {
    switch (role) {
      case "super_admin":
        return t("profilePage.roleSuperAdmin");
      case "admin":
        return t("profilePage.roleAdmin");
      case "author":
        return t("profilePage.roleAuthor");
      case "contributor":
        return t("profilePage.roleContributor");
      default:
        return t("profilePage.roleGuest");
    }
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title={t("profilePage.profile")}
        description={t("profilePage.profileDescription")}
      />
      <Card>
        <CardHeader className="items-center text-center">
          <div className="mb-1 flex justify-center">
            <div
              className="relative cursor-pointer"
              onClick={handleAvatarClick}
            >
              <Avatar className="size-20">
                {user?.avatar ? (
                  <img
                    src={user.avatar}
                    alt=""
                    className="size-full object-cover"
                  />
                ) : (
                  <AvatarFallback className="bg-muted text-muted-foreground text-2xl">
                    {user?.username?.charAt(0).toUpperCase()}
                  </AvatarFallback>
                )}
              </Avatar>
              <div className="bg-foreground/40 absolute inset-0 flex items-center justify-center rounded-full opacity-0 transition-opacity hover:opacity-100">
                {avatarLoading ? (
                  <Spinner className="size-6 text-background" />
                ) : (
                  <Camera className="size-6 text-background" />
                )}
              </div>
              <input
                type="file"
                id="avatar-upload"
                accept="image/*"
                className="hidden"
                onChange={handleAvatarChange}
              />
            </div>
          </div>
          <CardTitle className="text-lg">{user?.username}</CardTitle>
          <CardDescription>{user?.email}</CardDescription>
          <Badge variant="secondary" className="mt-1">
            {getRoleLabel(user?.role)}
          </Badge>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="profile" className="w-full">
            <TabsList className="w-full">
              <TabsTrigger value="profile">
                {t("profilePage.profileInfo")}
              </TabsTrigger>
              <TabsTrigger value="password">
                {t("profilePage.changePassword")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="profile">
              <form onSubmit={handleUpdateProfile} className="space-y-4">
                {success && (
                  <Alert variant="success">
                    <Check />
                    <AlertDescription className="text-current">
                      {success}
                    </AlertDescription>
                  </Alert>
                )}
                {error && (
                  <Alert variant="destructive">
                    <AlertDescription>{error}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-2">
                  <Label htmlFor="email">{t("profilePage.emailLabel")}</Label>
                  <div className="relative">
                    <Mail className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                    <Input
                      id="email"
                      type="email"
                      value={user?.email}
                      disabled
                      className="bg-muted pl-9"
                    />
                  </div>
                  <p className="text-muted-foreground text-[11px]">
                    {t("profilePage.changeEmailTip")}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    onClick={openEmailChangeDialog}
                  >
                    {t("profilePage.changeEmailButton")}
                  </Button>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="username">
                    {t("profilePage.usernameLabel")}
                  </Label>
                  <div className="relative">
                    <User className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                    <Input
                      id="username"
                      type="text"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      className="pl-9"
                      minLength={3}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="birthday">
                    {t("profilePage.birthdayLabel")}
                  </Label>
                  <div className="relative">
                    <Calendar className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                    <Input
                      id="birthday"
                      type="date"
                      value={birthday}
                      onChange={(e) => setBirthday(e.target.value)}
                      className="pl-9"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="bio">{t("profilePage.bioLabel")}</Label>
                  <Textarea
                    id="bio"
                    value={bio}
                    onChange={(e) => setBio(e.target.value)}
                    placeholder={t("profilePage.bioPlaceholder")}
                    rows={3}
                  />
                </div>

                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? (
                    <>
                      <Spinner className="size-4" />
                      {t("profilePage.saving")}
                    </>
                  ) : (
                    t("profilePage.saveChanges")
                  )}
                </Button>
              </form>
            </TabsContent>

            <TabsContent value="password">
              <form onSubmit={handleChangePassword} className="space-y-4">
                {success && (
                  <Alert variant="success">
                    <Check />
                    <AlertDescription className="text-current">
                      {success}
                    </AlertDescription>
                  </Alert>
                )}
                {error && (
                  <Alert variant="destructive">
                    <AlertDescription>{error}</AlertDescription>
                  </Alert>
                )}

                <PasswordField
                  id="oldPassword"
                  label={t("profilePage.currentPassword")}
                  value={oldPassword}
                  onChange={setOldPassword}
                  visible={showOldPassword}
                  onToggleVisibility={() =>
                    setShowOldPassword(!showOldPassword)
                  }
                  autoComplete="current-password"
                />

                <PasswordField
                  id="newPassword"
                  label={t("profilePage.newPassword")}
                  value={newPassword}
                  onChange={setNewPassword}
                  visible={showNewPassword}
                  onToggleVisibility={() =>
                    setShowNewPassword(!showNewPassword)
                  }
                  autoComplete="new-password"
                  minLength={6}
                />

                <PasswordField
                  id="confirmPassword"
                  label={t("profilePage.confirmPassword")}
                  value={confirmPassword}
                  onChange={setConfirmPassword}
                  visible={showConfirmPassword}
                  onToggleVisibility={() =>
                    setShowConfirmPassword(!showConfirmPassword)
                  }
                  autoComplete="new-password"
                />

                <Button
                  type="submit"
                  className="w-full"
                  disabled={passwordLoading}
                >
                  {passwordLoading ? (
                    <>
                      <Spinner className="size-4" />
                      {t("profilePage.saving")}
                    </>
                  ) : (
                    t("profilePage.changePassword")
                  )}
                </Button>
              </form>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* Email change confirmation dialog */}
      <AlertDialog open={showEmailDialog} onOpenChange={setShowEmailDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("profilePage.changeEmailDialog")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("profilePage.changeEmailDescription", {
                smtpEnabled: user?.emailVerified
                  ? t("profilePage.smtpEnabled")
                  : t("profilePage.smtpDisabled"),
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2 py-1">
            <Label htmlFor="newEmailInput">{t("profilePage.newEmail")}</Label>
            <div className="relative">
              <Mail className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                id="newEmailInput"
                type="email"
                placeholder={t("profilePage.enterNewEmail")}
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                className="pl-9"
              />
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={emailLoading}>
              {t("common.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                handleUpdateEmail(e);
              }}
              disabled={emailLoading || !newEmail}
            >
              {emailLoading ? (
                <>
                  <Spinner className="size-4" />
                  {t("profilePage.saving")}
                </>
              ) : (
                t("profilePage.confirmChange")
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Avatar cropping */}
      {showCropper && selectedAvatarFile && (
        <ImageCropper
          file={selectedAvatarFile}
          circle={true}
          aspect={1}
          outputWidth={400}
          outputHeight={400}
          onCancel={() => {
            setShowCropper(false);
            setSelectedAvatarFile(null);
          }}
          onCrop={handleAvatarCrop}
        />
      )}
    </div>
  );
}
