const screenshotDetails: Record<string, { label: string; src: string }> = {
  overview: { label: "Overview", src: "/public-assets/img/overview.JPG" },
  buildings: { label: "Buildings", src: "/public-assets/img/buildings.JPG" },
  shipyard: { label: "Shipyard", src: "/public-assets/img/shipyard.JPG" },
  empire: { label: "Empire", src: "/public-assets/img/empire.JPG" },
  battleship_1280x1024: { label: "", src: "/public-assets/img/wallpapers/battleship_1280x1024.jpg" },
  destroyer_1280x1024: { label: "", src: "/public-assets/img/wallpapers/destroyer_1280x1024.jpg" }
};

export function LegacyPublicScreenshot({ search }: { search: string }) {
  const requested = new URLSearchParams(search).get("pic") ?? "overview";
  const screenshot = screenshotDetails[requested] ?? screenshotDetails.overview;
  return (
    <main className="legacy-public-screenshot-detail">
      <p className="bildUeberschrift">{screenshot.label}</p>
      <a href="/screenshots">
        <img alt="" src={screenshot.src} />
      </a>
    </main>
  );
}
