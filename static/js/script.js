function jsScript() {
    // reload page when user clicks on "show all" or "show filtered"
    document.getElementById("filters-div").addEventListener("click", function(e) {
        if(e.target.id === "show-all-btn") {
            window.open(window.location.href.split('?')[0],'_self');
        } else {
            window.open('?filtered=true','_self');
        }
    });

    // highlight the proper filters button and update info text
    let params = new URLSearchParams(document.location.search);
    let filtered = params.get("filtered");
    if (filtered === "true") {
        document.getElementById("show-filtered-btn").classList.add("active");
        document.getElementById("show-all-btn").classList.remove("active");
        document.getElementById("channels-info-span").textContent="# of channels with new videos:";
    } else {
        document.getElementById("show-all-btn").classList.add("active");
        document.getElementById("show-filtered-btn").classList.remove("active");
        document.getElementById("channels-info-span").textContent="# of channels:";
        document.querySelectorAll(".mark-as-viewed").forEach(el => el.remove());
    }
}

// remove table row when user clicks on "mark as viewed"
async function callURL(url, serverBasepath) {
    try {
        const trID = event.target.closest("tr").id;
        let totVids = Number(document.getElementById("tot-channels").textContent);
        await fetch(serverBasepath + "/mark-as-viewed", {
            method: 'POST',
            body: JSON.stringify({url: url})
        });
        document.getElementById(trID).remove();
        document.getElementById("tot-channels").textContent = totVids - 1;
    } catch (e) {
        console.log(e);
        return e;
    }
}
