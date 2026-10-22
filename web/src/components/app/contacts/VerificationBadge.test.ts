import { describe, expect, it } from "vitest";
import { verificationSourceLabel, verificationTitle } from "./VerificationBadge";

describe("verification provider attribution", () => {
    it.each(["CleanMyList", "MillionVerifier"])("names %s on provider verdicts", (name) => {
        expect(verificationTitle({
            verification_status: "valid",
            verification_source: "provider",
            verification_provider: name.toLowerCase(),
        })).toBe("Deliverable · checked through Pramaan");
    });

    it("keeps the generic label for a verifier it has no name for", () => {
        expect(verificationTitle({
            verification_status: "valid",
            verification_source: "provider",
            verification_provider: "some-service",
        })).toBe("Deliverable · checked through Pramaan");
    });

    it("says when a re-check is waiting on the current verdict", () => {
        expect(verificationTitle({
            verification_status: "invalid",
            verification_source: "provider",
            verification_provider: "millionverifier",
            verification_requested_at: new Date("2026-09-22T10:00:00Z"),
        })).toBe("Undeliverable · checked through Pramaan · re-check queued");
    });

    it("names the built-in check and an imported vocabulary", () => {
        expect(verificationSourceLabel("probe", "builtin")).toBe("checked through Pramaan");
        expect(verificationSourceLabel("imported", "zerobounce", "ZeroBounce")).toBe("imported with the list");
        expect(verificationSourceLabel("", "")).toBe("");
    });
});

it('never presents an unknown check as a verified remote result or confidence claim',()=>{
 expect(verificationTitle({verification_status:'unknown',verification_confidence:30,verification_source:'probe',verification_provider:'builtin'})).toBe('Not verified yet');
 expect(verificationTitle({verification_status:'unknown',verification_confidence:30,verification_source:'probe',verification_requested_at:new Date()})).toBe('Not verified yet · re-check queued');
});
