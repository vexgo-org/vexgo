import { useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import type { User } from "@/types";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { PageHeader } from "@/components/PageHeader";
import { Spinner } from "@/components/ui/spinner";
import { Check, Shield } from "lucide-react";

export function SettingsPage() {
  const { user, updateUser } = useAuth();
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);

  // Privacy settings - initialized only from the user object (fetched from the backend)
  const [profileVisibility, setProfileVisibility] = useState<
    "public" | "private"
  >(() => {
    return (user?.profile_visibility as "public" | "private") || "public";
  });
  const [hideEmail, setHideEmail] = useState(() => {
    return user?.hide_email || false;
  });
  const [hideBirthday, setHideBirthday] = useState(() => {
    return user?.hide_birthday || false;
  });
  const [hideBio, setHideBio] = useState(() => {
    return user?.hide_bio || false;
  });

  const [success, setSuccess] = useState("");

  const handleSave = async () => {
    setLoading(true);
    setError("");

    try {
      // Save the privacy settings to the server
      const response = await getVexGoAPI().putAuthSettings({
        profile_visibility: profileVisibility,
        hide_email: hideEmail,
        hide_birthday: hideBirthday,
        hide_bio: hideBio,
      });

      // Update the local user info
      if (response.data.user) {
        updateUser(response.data.user as User);
      }

      setSuccess(t("settings.saveSuccess"));
      setTimeout(() => setSuccess(""), 3000);
    } catch (err: unknown) {
      const errorMessage =
        err instanceof Error ? err.message : t("settings.saveFailed");
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const [error, setError] = useState("");

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title={t("settings.title")}
        description={t("settings.privacySettingsDesc")}
      />

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

      {/* Display preferences (theme, language) live in the console's utility
          bar, so this page only owns what the account itself controls. */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="w-5 h-5" />
            {t("settings.privacySettings")}
          </CardTitle>
          <CardDescription>{t("settings.privacySettingsDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label>{t("settings.profileVisibility")}</Label>
            <Select
              value={profileVisibility}
              onValueChange={(value: "public" | "private") =>
                setProfileVisibility(value)
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t("settings.profileVisibility")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="public">{t("settings.public")}</SelectItem>
                <SelectItem value="private">{t("settings.private")}</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-sm text-muted-foreground">
              {profileVisibility === "public"
                ? t("settings.publicDesc")
                : t("settings.privateDesc")}
            </p>
          </div>

          <div className="space-y-4 pt-2">
            <h3 className="text-sm font-medium">
              {t("settings.personalInfoVisibility")}
            </h3>

            <div className="flex items-center justify-between">
              <div>
                <Label>{t("settings.hideEmail")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("settings.hideEmailDesc")}
                </p>
              </div>
              <Switch checked={hideEmail} onCheckedChange={setHideEmail} />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <Label>{t("settings.hideBirthday")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("settings.hideBirthdayDesc")}
                </p>
              </div>
              <Switch
                checked={hideBirthday}
                onCheckedChange={setHideBirthday}
              />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <Label>{t("settings.hideBio")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("settings.hideBioDesc")}
                </p>
              </div>
              <Switch checked={hideBio} onCheckedChange={setHideBio} />
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={loading}>
          {loading ? (
            <>
              <Spinner className="mr-2" />
              {t("common.saving")}
            </>
          ) : (
            t("settings.save")
          )}
        </Button>
      </div>
    </div>
  );
}
