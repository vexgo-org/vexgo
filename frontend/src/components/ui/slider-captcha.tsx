import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import GoCaptcha from "go-captcha-react";
import { Button } from "./button";
import { RefreshCw, CheckCircle, X } from "lucide-react";
import { useTranslation } from "@/lib/I18nContext";

interface CaptchaResponse {
  id: string;
  token: string;
  thumbX: number;
  thumbY: number;
  thumbWidth: number;
  thumbHeight: number;
  image: string;
  thumb: string;
  expires_at: string;
}

interface SlideCaptchaRef {
  reset: () => void;
  clear: () => void;
  refresh: () => void;
  close: () => void;
}

interface SliderCaptchaProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (captchaData: {
    id: string;
    token: string;
    x: number;
    y: number;
  }) => void;
}

export function SliderCaptcha({
  isOpen,
  onClose,
  onSuccess,
}: SliderCaptchaProps) {
  const { t } = useTranslation();
  const [captchaData, setCaptchaData] = useState<CaptchaResponse | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [isVerified, setIsVerified] = useState(false);
  const [error, setError] = useState("");
  const slideRef = useRef<SlideCaptchaRef>(null);

  // The Slide component resets its internal tile position whenever the data/
  // config/events prop identities change, so memoize them and only hand over
  // new identities when a fresh captcha is loaded; otherwise every unrelated
  // re-render (e.g. setIsVerified) would snap the tile back to its start.
  const slideData = useMemo(
    () =>
      captchaData
        ? {
            thumbX: captchaData.thumbX,
            thumbY: captchaData.thumbY,
            thumbWidth: captchaData.thumbWidth,
            thumbHeight: captchaData.thumbHeight,
            image: captchaData.image,
            thumb: captchaData.thumb,
          }
        : {
            thumbX: 0,
            thumbY: 0,
            thumbWidth: 0,
            thumbHeight: 0,
            image: "",
            thumb: "",
          },
    [captchaData],
  );
  const slideConfig = useMemo(
    () => ({
      width: 320,
      height: 160,
      title: t("sliderCaptcha.dragHint"),
    }),
    [t],
  );

  // Generate a captcha
  const generateCaptcha = useCallback(async () => {
    try {
      setIsVerifying(true);
      setError("");
      setIsVerified(false);

      const response = await fetch("/api/captcha");
      if (!response.ok) {
        throw new Error(t("sliderCaptcha.fetchFailed"));
      }

      const data: CaptchaResponse = await response.json();
      setCaptchaData(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t("sliderCaptcha.fetchFailed"),
      );
    } finally {
      setIsVerifying(false);
    }
  }, [t]);

  // Generate a captcha when the dialog opens
  useEffect(() => {
    if (isOpen) {
      generateCaptcha();
    }
  }, [isOpen, generateCaptcha]);

  // Verify the dropped position on the backend and report success upward
  const verifyCaptcha = useCallback(
    async (x: number, y: number) => {
      if (!captchaData) return;

      try {
        setIsVerifying(true);
        setError("");

        const response = await fetch("/api/captcha/verify", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            id: captchaData.id,
            token: captchaData.token,
            x,
            y,
          }),
        });

        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.error || t("sliderCaptcha.retryError"));
        }

        const data = await response.json();

        if (data.success) {
          setIsVerified(true);
          onSuccess({ id: captchaData.id, token: captchaData.token, x, y });
          // Close the dialog after a successful verification
          setTimeout(() => {
            onClose();
          }, 500);
        } else {
          throw new Error(data.message || t("sliderCaptcha.retryError"));
        }
      } catch (err) {
        setError(
          err instanceof Error ? err.message : t("sliderCaptcha.retryError"),
        );
        slideRef.current?.reset();
        // Refresh the captcha after a failed verification
        setTimeout(() => {
          generateCaptcha();
        }, 1000);
      } finally {
        setIsVerifying(false);
      }
    },
    [captchaData, onSuccess, onClose, generateCaptcha, t],
  );

  const slideEvents = useMemo(
    () => ({
      confirm: (point: { x: number; y: number }) => {
        verifyCaptcha(point.x, point.y);
      },
      refresh: () => {
        generateCaptcha();
      },
    }),
    [verifyCaptcha, generateCaptcha],
  );

  // Do not render anything if the dialog is closed
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-lg w-full max-w-md p-6">
        <div className="flex justify-between items-center mb-4">
          <h3 className="text-lg font-medium text-gray-900">
            {t("sliderCaptcha.title")}
          </h3>
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="h-8 w-8 p-0"
            title={t("sliderCaptcha.closeButton")}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        {error && !isVerified && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded text-sm text-red-600">
            {t("sliderCaptcha.error")}
          </div>
        )}

        {isVerified && (
          <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded text-sm text-green-600 flex items-center">
            <CheckCircle className="h-4 w-4 mr-2" />
            {t("sliderCaptcha.successHint")}
          </div>
        )}

        {captchaData && (
          <div className="space-y-4">
            <GoCaptcha.Slide
              ref={slideRef}
              data={slideData}
              config={slideConfig}
              events={slideEvents}
            />

            {/* Refresh button */}
            <div className="flex justify-center">
              <Button
                variant="ghost"
                size="sm"
                onClick={generateCaptcha}
                disabled={isVerifying}
                className="text-sm"
              >
                <RefreshCw
                  className={`h-4 w-4 mr-1 ${isVerifying ? "animate-spin" : ""}`}
                />
                {t("sliderCaptcha.refreshButton")}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
