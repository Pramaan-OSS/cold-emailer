// Neutral artwork accompanies the tour text; no historical product screenshots.
import { Logo } from "@/components/svg";
import type { SceneId } from "./tourSteps";
export default function TourScene({ id: _id }: { id: SceneId }) {
    return <div className="absolute inset-0 flex items-center justify-center" aria-hidden="true"><Logo width={80} height={80} /></div>;
}
