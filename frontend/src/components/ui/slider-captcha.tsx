import React, { useCallback, useState, useEffect, useRef } from "react";
import { Button } from "./button";
import { RefreshCw, CheckCircle, X } from "lucide-react";
import { useTranslation } from "@/lib/I18nContext";

interface SliderCaptchaProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (captchaData: { id: string; token: string; x: number }) => void;
  onVerify?: () => void;
}

export function SliderCaptcha({
  isOpen,
  onClose,
  onSuccess,
}: SliderCaptchaProps) {
  const { t } = useTranslation();
  const [captchaData, setCaptchaData] = useState<{
    id: string;
    token: string;
    bg_image: string;
    puzzle_img: string;
    y: number;
    expires_at: string;
  } | null>(null);
  const [isVerifying, setIsVerifying] = useState(false);
  const [isVerified, setIsVerified] = useState(false);
  const [error, setError] = useState("");
  const [isDragging, setIsDragging] = useState(false);
  const [sliderPosition, setSliderPosition] = useState(0);
  const canClose = useState(false);
  const sliderRef = useRef<HTMLDivElement>(null);
  const trackRef = useRef<HTMLDivElement>(null);
  const startXRef = useRef(0);
  const currentXRef = useRef(0);
  const isDraggingRef = useRef(false);
  const sliderPositionRef = useRef(0);

  // Keep refs in sync with state
  useEffect(() => {
    isDraggingRef.current = isDragging;
    sliderPositionRef.current = sliderPosition;
  }, [isDragging, sliderPosition]);

  // Generate a captcha
  const generateCaptcha = useCallback(async () => {
    try {
      setIsVerifying(true);
      setError("");
      setSliderPosition(0);
      setIsVerified(false);

      const response = await fetch("/api/captcha");
      if (!response.ok) {
        throw new Error(t("sliderCaptcha.fetchFailed"));
      }

      const data = await response.json();
      setCaptchaData(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t("sliderCaptcha.fetchFailed"),
      );
    } finally {
      setIsVerifying(false);
    }
  }, []);

  // Generate a captcha when the dialog opens
  useEffect(() => {
    if (isOpen) {
      generateCaptcha();
    }
  }, [isOpen, generateCaptcha]);

  // Verify the puzzle
  const verifyCaptcha = useCallback(
    async (x: number) => {
      if (!captchaData) return;

      try {
        setIsVerifying(true);
        setError("");

        // Calculate the x-coordinate of the puzzle piece's left edge
        // Slider width is 40px, puzzle piece width is 60px
        // Slider center = x + 20; the puzzle piece center should align with the slider center
        // Puzzle left edge = slider center - 30 = x + 20 - 30 = x - 10
        const puzzleX = Math.round(x - 10);
        console.log("Slider position:", x, "puzzle left edge:", puzzleX);

        // Call the backend verify endpoint
        const response = await fetch("/api/captcha/verify", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            id: captchaData.id,
            token: captchaData.token,
            x: puzzleX,
          }),
        });

        if (!response.ok) {
          const errorData = await response.json();
          throw new Error(errorData.error || t("sliderCaptcha.retryError"));
        }

        const data = await response.json();

        if (data.success) {
          // Verification succeeded
          setIsVerified(true);
          onSuccess({
            id: captchaData.id,
            token: captchaData.token,
            x: puzzleX,
          });
          // Close the dialog after a successful verification
          setTimeout(() => {
            onClose();
          }, 500);
        } else {
          // Verification failed
          throw new Error(data.message || t("sliderCaptcha.retryError"));
        }
      } catch (err) {
        setError(
          err instanceof Error ? err.message : t("sliderCaptcha.retryError"),
        );
        // Refresh the captcha after a failed verification
        setTimeout(() => {
          generateCaptcha();
        }, 1000);
      } finally {
        setIsVerifying(false);
      }
    },
    [captchaData, onSuccess, onClose, generateCaptcha],
  );

  // Handle slider dragging
  const handleMouseDown = (e: React.MouseEvent) => {
    if (isVerified) return;
    setIsDragging(true);
    startXRef.current = e.clientX;
    currentXRef.current = sliderPosition;
  };

  // Attach global mouse event listeners
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isDraggingRef.current || !trackRef.current) return;

      const trackWidth = trackRef.current.offsetWidth;
      const sliderWidth = 40;
      const deltaX = e.clientX - startXRef.current;
      let newPosition = currentXRef.current + deltaX;

      newPosition = Math.max(
        0,
        Math.min(newPosition, trackWidth - sliderWidth),
      );

      setSliderPosition(newPosition);
    };

    const handleMouseUp = () => {
      if (!isDraggingRef.current) return;
      setIsDragging(false);

      // Verify the puzzle position
      if (sliderPositionRef.current > 0) {
        verifyCaptcha(sliderPositionRef.current);
      }
    };

    if (isDragging) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);

      return () => {
        document.removeEventListener("mousemove", handleMouseMove);
        document.removeEventListener("mouseup", handleMouseUp);
      };
    }
  }, [isDragging, verifyCaptcha]);

  // Handle touch events
  const handleTouchStart = (e: React.TouchEvent) => {
    if (isVerified) return;
    setIsDragging(true);
    startXRef.current = e.touches[0].clientX;
    currentXRef.current = sliderPosition;
  };

  // Attach global touch event listeners
  useEffect(() => {
    const handleTouchMove = (e: TouchEvent) => {
      if (!isDraggingRef.current || !trackRef.current) return;

      e.preventDefault();

      const trackWidth = trackRef.current.offsetWidth;
      const sliderWidth = 40;
      const deltaX = e.touches[0].clientX - startXRef.current;
      let newPosition = currentXRef.current + deltaX;

      newPosition = Math.max(
        0,
        Math.min(newPosition, trackWidth - sliderWidth),
      );

      setSliderPosition(newPosition);
    };

    const handleTouchEnd = () => {
      if (!isDraggingRef.current) return;
      setIsDragging(false);

      // Verify the puzzle position
      if (sliderPositionRef.current > 0) {
        verifyCaptcha(sliderPositionRef.current);
      }
    };

    if (isDragging) {
      document.addEventListener("touchmove", handleTouchMove, {
        passive: false,
      });
      document.addEventListener("touchend", handleTouchEnd);

      return () => {
        document.removeEventListener("touchmove", handleTouchMove);
        document.removeEventListener("touchend", handleTouchEnd);
      };
    }
  }, [isDragging, verifyCaptcha]);

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
            onClick={() => {
              if (canClose) {
                onClose();
              }
            }}
            disabled={!canClose}
            className="h-8 w-8 p-0"
            title={
              canClose
                ? t("sliderCaptcha.closeButton")
                : t("sliderCaptcha.closeButtonDisabled")
            }
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
            {/* Puzzle area */}
            <div
              className="relative bg-gray-100 rounded overflow-hidden flex justify-center items-center"
              style={{ height: "160px" }}
            >
              {/* Background image */}
              <div
                className="relative"
                style={{ width: "320px", height: "160px" }}
              >
                <img
                  src={captchaData.bg_image}
                  alt={t("sliderCaptcha.backgroundAlt")}
                  className="w-full h-full"
                />

                {/* Puzzle piece - aligned with the slider center */}
                <div
                  className="absolute pointer-events-none"
                  style={{
                    // Slider width is 40px, puzzle piece width is 60px
                    // Slider center = sliderPosition + 20
                    // The puzzle piece center should align with the slider center
                    // Puzzle left edge = slider center - 30 = sliderPosition - 10
                    left: `${sliderPosition - 10}px`,
                    top: `${captchaData.y}px`,
                  }}
                >
                  <img
                    src={captchaData.puzzle_img}
                    alt={t("sliderCaptcha.puzzleAlt")}
                    style={{ width: "60px", height: "60px" }}
                  />
                </div>
              </div>
            </div>

            {/* Slider track */}
            <div
              ref={trackRef}
              className="relative h-10 bg-gray-200 rounded-full overflow-hidden"
            >
              {/* Progress bar - follows the slider center */}
              <div
                className="absolute top-0 left-0 h-full bg-blue-500 transition-all duration-75"
                style={{ width: `${sliderPosition + 20}px` }}
              />

              {/* Slider */}
              <div
                ref={sliderRef}
                className={`absolute top-1/2 w-10 h-10 bg-white rounded-full shadow-lg flex items-center justify-center cursor-pointer transform -translate-y-1/2 transition-all duration-75 ${
                  isDragging ? "scale-105 shadow-xl" : "hover:scale-105"
                } ${isVerified ? "bg-green-500" : ""}`}
                style={{ left: `${sliderPosition}px` }}
                onMouseDown={handleMouseDown}
                onTouchStart={handleTouchStart}
              >
                {isVerified ? (
                  <CheckCircle className="h-5 w-5 text-white" />
                ) : (
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      d="M5 12H19"
                      stroke="#4B5563"
                      strokeWidth="2"
                      strokeLinecap="round"
                    />
                    <path
                      d="M12 5L19 12L12 19"
                      stroke="#4B5563"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                )}
              </div>
            </div>

            {/* Hint text */}
            <div className="text-center text-sm text-gray-600">
              {isVerified ? (
                <span className="text-green-600 font-medium">
                  {t("sliderCaptcha.successHint")}
                </span>
              ) : (
                t("sliderCaptcha.dragHint")
              )}
            </div>

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
