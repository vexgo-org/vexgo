import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import GoCaptcha from "go-captcha-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/I18nContext";
import { Spinner } from "./spinner";

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
  const slideRef = useRef<SlideCaptchaRef>(null);

  // The Slide component resets its internal tile position whenever the data/
  // config/events prop identities change, so memoize them and only hand over
  // new identities when a fresh captcha is loaded; otherwise every unrelated
  // re-render would snap the tile back to its start.
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
      const response = await fetch("/api/captcha");
      if (!response.ok) {
        throw new Error(t("sliderCaptcha.fetchFailed"));
      }

      const data: CaptchaResponse = await response.json();
      setCaptchaData(data);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : t("sliderCaptcha.fetchFailed");
      toast.error(message);
    }
  }, [t]);

  // Generate a captcha when the overlay opens
  useEffect(() => {
    if (isOpen) {
      generateCaptcha();
    }
  }, [isOpen, generateCaptcha]);

  // Close on Escape; the captcha frame is the only element on the overlay
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  // Verify the dropped position on the backend and report success upward
  const verifyCaptcha = useCallback(
    async (x: number, y: number) => {
      if (!captchaData) return;

      try {
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
          toast.success(t("sliderCaptcha.success"));
          onSuccess({ id: captchaData.id, token: captchaData.token, x, y });
          // Close the overlay after a successful verification
          setTimeout(() => {
            onClose();
          }, 500);
        } else {
          throw new Error(data.message || t("sliderCaptcha.retryError"));
        }
      } catch (err) {
        const message =
          err instanceof Error ? err.message : t("sliderCaptcha.retryError");
        toast.error(message);
        slideRef.current?.reset();
        // Refresh the captcha after a failed verification
        setTimeout(() => {
          generateCaptcha();
        }, 1000);
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
      close: () => {
        onClose();
      },
    }),
    [verifyCaptcha, generateCaptcha, onClose],
  );

  // Do not render anything if the overlay is closed
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      {captchaData ? (
        <div className="slider-captcha">
          <GoCaptcha.Slide
            ref={slideRef}
            data={slideData}
            config={slideConfig}
            events={slideEvents}
          />
        </div>
      ) : (
        <div className="flex h-40 w-80 items-center justify-center rounded-lg bg-card">
          <Spinner className="size-6" />
        </div>
      )}
    </div>
  );
}
